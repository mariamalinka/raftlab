// Package wal is the append-only write-ahead log behind each Raft node.
//
// Every record carries a CRC-32C checksum so torn or corrupted tails are
// detected on recovery and cut off. The log is written by the node itself
// (not a library) so the fsync policy and lost-write experiments measure our
// own durability discipline.
//
// Layout of one record: crc32c(4) | length(4) | type(1) | payload(length-1).
// The checksum covers type and payload.
package wal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"

	"raftkv/internal/raft"
)

const (
	recHardState byte = 1
	recEntry     byte = 2
	recTruncate  byte = 3 // drop every entry with Index >= payload
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

type Options struct {
	// NoSync turns fsync off: Sync() still hands data to the OS but never
	// forces it to disk. Labelled "no-fsync" runs only.
	NoSync bool
	// Lazy keeps unsynced writes in process memory until Sync(), so a kill -9
	// loses them. This stands in for LazyFS, which simulates a machine crash
	// dropping the page cache.
	Lazy bool
}

type WAL struct {
	mu        sync.Mutex
	dir       string
	opts      Options
	f         *os.File
	size      int64
	buf       []byte // records not yet handed to the OS (Lazy mode, or scratch)
	lastIndex uint64
	snapIndex uint64
}

func walPath(dir string) string  { return filepath.Join(dir, "wal.log") }
func snapPath(dir string) string { return filepath.Join(dir, "snapshot") }

// Open opens (creating if needed) the log in dir.
func Open(dir string, opts Options) (*WAL, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(walPath(dir), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	return &WAL{dir: dir, opts: opts, f: f}, nil
}

func (w *WAL) Load() (raft.HardState, raft.Snapshot, []raft.Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var hs raft.HardState
	snap, err := readSnapshot(snapPath(w.dir))
	if err != nil {
		return hs, snap, nil, err
	}
	data, err := io.ReadAll(io.NewSectionReader(w.f, 0, 1<<62))
	if err != nil {
		return hs, snap, nil, err
	}
	var ents []raft.Entry
	off := 0
	for {
		typ, payload, n, ok := decodeRecord(data[off:])
		if !ok {
			break
		}
		switch typ {
		case recHardState:
			hs.Term = binary.LittleEndian.Uint64(payload[0:])
			hs.Vote = binary.LittleEndian.Uint64(payload[8:])
		case recEntry:
			e := decodeEntry(payload)
			if e.Index <= snap.Index {
				break
			}
			if len(ents) > 0 && e.Index != ents[len(ents)-1].Index+1 {
				return hs, snap, nil, fmt.Errorf("wal: gap at index %d after %d", e.Index, ents[len(ents)-1].Index)
			}
			ents = append(ents, e)
		case recTruncate:
			from := binary.LittleEndian.Uint64(payload)
			for len(ents) > 0 && ents[len(ents)-1].Index >= from {
				ents = ents[:len(ents)-1]
			}
		}
		off += n
	}
	if off < len(data) {
		// Torn or corrupt tail: cut it off so later appends start clean.
		if err := w.f.Truncate(int64(off)); err != nil {
			return hs, snap, nil, err
		}
	}
	if _, err := w.f.Seek(int64(off), io.SeekStart); err != nil {
		return hs, snap, nil, err
	}
	w.size = int64(off)
	w.snapIndex = snap.Index
	w.lastIndex = snap.Index
	if len(ents) > 0 {
		w.lastIndex = ents[len(ents)-1].Index
	}
	return hs, snap, ents, nil
}

func decodeRecord(b []byte) (typ byte, payload []byte, n int, ok bool) {
	if len(b) < 9 {
		return 0, nil, 0, false
	}
	sum := binary.LittleEndian.Uint32(b[0:])
	length := int(binary.LittleEndian.Uint32(b[4:]))
	if length < 1 || 8+length > len(b) {
		return 0, nil, 0, false
	}
	body := b[8 : 8+length]
	if crc32.Checksum(body, crcTable) != sum {
		return 0, nil, 0, false
	}
	return body[0], body[1:], 8 + length, true
}

func appendRecord(buf []byte, typ byte, payload []byte) []byte {
	start := len(buf)
	buf = append(buf, 0, 0, 0, 0, 0, 0, 0, 0, typ)
	buf = append(buf, payload...)
	body := buf[start+8:]
	binary.LittleEndian.PutUint32(buf[start:], crc32.Checksum(body, crcTable))
	binary.LittleEndian.PutUint32(buf[start+4:], uint32(len(body)))
	return buf
}

func encodeHardState(hs raft.HardState) []byte {
	p := make([]byte, 16)
	binary.LittleEndian.PutUint64(p[0:], hs.Term)
	binary.LittleEndian.PutUint64(p[8:], hs.Vote)
	return p
}

func encodeEntry(dst []byte, e raft.Entry) []byte {
	dst = binary.LittleEndian.AppendUint64(dst, e.Index)
	dst = binary.LittleEndian.AppendUint64(dst, e.Term)
	dst = append(dst, byte(e.Type))
	return append(dst, e.Data...)
}

func decodeEntry(p []byte) raft.Entry {
	e := raft.Entry{
		Index: binary.LittleEndian.Uint64(p[0:]),
		Term:  binary.LittleEndian.Uint64(p[8:]),
		Type:  raft.EntryType(p[16]),
	}
	if len(p) > 17 {
		e.Data = append([]byte(nil), p[17:]...)
	}
	return e
}

// write queues records; outside Lazy mode they go straight to the OS, so a
// process crash keeps them even without fsync.
func (w *WAL) write(rec []byte) error {
	w.size += int64(len(rec))
	if w.opts.Lazy {
		w.buf = append(w.buf, rec...)
		return nil
	}
	_, err := w.f.Write(rec)
	return err
}

func (w *WAL) SetHardState(hs raft.HardState) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.write(appendRecord(nil, recHardState, encodeHardState(hs)))
}

func (w *WAL) Append(ents []raft.Entry) error {
	if len(ents) == 0 {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var rec []byte
	if ents[0].Index <= w.lastIndex {
		rec = appendRecord(rec, recTruncate, binary.LittleEndian.AppendUint64(nil, ents[0].Index))
	} else if ents[0].Index != w.lastIndex+1 {
		return fmt.Errorf("wal: append at %d leaves a gap after %d", ents[0].Index, w.lastIndex)
	}
	var p []byte
	for _, e := range ents {
		p = encodeEntry(p[:0], e)
		rec = appendRecord(rec, recEntry, p)
	}
	w.lastIndex = ents[len(ents)-1].Index
	return w.write(rec)
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.syncLocked()
}

func (w *WAL) syncLocked() error {
	if len(w.buf) > 0 {
		if _, err := w.f.Write(w.buf); err != nil {
			return err
		}
		w.buf = w.buf[:0]
	}
	if w.opts.NoSync {
		return nil
	}
	return w.f.Sync()
}

func (w *WAL) Size() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size
}

func (w *WAL) SaveSnapshot(snap raft.Snapshot, hs raft.HardState, ents []raft.Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.syncLocked(); err != nil {
		return err
	}
	if err := writeSnapshot(snapPath(w.dir), snap); err != nil {
		return err
	}
	// Rewrite the log without the compacted prefix.
	rec := appendRecord(nil, recHardState, encodeHardState(hs))
	var p []byte
	for _, e := range ents {
		p = encodeEntry(p[:0], e)
		rec = appendRecord(rec, recEntry, p)
	}
	tmp := walPath(w.dir) + ".tmp"
	if err := writeFileSync(tmp, rec); err != nil {
		return err
	}
	if err := os.Rename(tmp, walPath(w.dir)); err != nil {
		return err
	}
	if err := syncDir(w.dir); err != nil {
		return err
	}
	f, err := os.OpenFile(walPath(w.dir), os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		f.Close()
		return err
	}
	w.f.Close()
	w.f = f
	w.size = int64(len(rec))
	w.snapIndex = snap.Index
	w.lastIndex = snap.Index
	if len(ents) > 0 {
		w.lastIndex = ents[len(ents)-1].Index
	}
	return nil
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

// Snapshot file: crc32c(4) | index(8) | term(8) | data.
func writeSnapshot(path string, snap raft.Snapshot) error {
	body := binary.LittleEndian.AppendUint64(nil, snap.Index)
	body = binary.LittleEndian.AppendUint64(body, snap.Term)
	body = append(body, snap.Data...)
	out := binary.LittleEndian.AppendUint32(nil, crc32.Checksum(body, crcTable))
	out = append(out, body...)
	tmp := path + ".tmp"
	if err := writeFileSync(tmp, out); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func readSnapshot(path string) (raft.Snapshot, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return raft.Snapshot{}, nil
	}
	if err != nil {
		return raft.Snapshot{}, err
	}
	if len(b) < 20 || crc32.Checksum(b[4:], crcTable) != binary.LittleEndian.Uint32(b) {
		return raft.Snapshot{}, fmt.Errorf("wal: corrupt snapshot %s", path)
	}
	return raft.Snapshot{
		Index: binary.LittleEndian.Uint64(b[4:]),
		Term:  binary.LittleEndian.Uint64(b[12:]),
		Data:  bytes.Clone(b[20:]),
	}, nil
}

func writeFileSync(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
