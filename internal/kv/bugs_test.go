package kv

import (
	"log/slog"
	"testing"

	"raftkv/internal/raft"
)

// nodedup: a client retries a write with the same (client ID, sequence
// number) when it cannot tell whether the first attempt committed. If both
// attempts commit, the state machine must apply the write once. Here client
// A's Put commits, client B overwrites it, then A's retry commits too; the
// buggy store applies the retry and brings A's stale value back.
//
//	go test ./internal/kv -run TestBugNoDedup                         # passes
//	RAFTKV_BUGS=nodedup go test -tags bugs ./internal/kv -run TestBugNoDedup  # fails
func TestBugNoDedup(t *testing.T) {
	s := &Store{
		log:      slog.New(slog.DiscardHandler),
		data:     map[string]string{},
		sessions: map[uint64]session{},
		waiters:  map[uint64][]waiter{},
	}
	cmds := []command{
		{op: opPut, clientID: 1, seq: 1, key: "k", value: "a"},
		{op: opPut, clientID: 2, seq: 1, key: "k", value: "b"},
		{op: opPut, clientID: 1, seq: 1, key: "k", value: "a"}, // A's retry
	}
	for i, c := range cmds {
		s.applyEntry(&raft.Entry{Index: uint64(i + 1), Term: 1, Type: raft.EntryNormal, Data: c.encode()})
	}
	if got := s.data["k"]; got != "b" {
		t.Fatalf("k = %q after A's retried write, want %q: the duplicate was applied twice", got, "b")
	}
}
