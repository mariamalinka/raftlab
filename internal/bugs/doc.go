// Package bugs holds the bug-injection switches for the RQ3 study.
//
// Each switch re-introduces one known Raft/KV bug from the research plan's
// correctness table. In a normal build (no "bugs" build tag) every switch is
// the constant false, so the compiler removes the buggy branches entirely.
// Build with `-tags bugs` and set RAFTKV_BUGS (comma-separated names) to turn
// individual bugs on at startup.
package bugs

// Names lists every injectable bug, in the order of the plan's table.
var Names = []string{
	"figure8",        // commit previous-term entries by counting replicas
	"localread",      // serve reads on the leader without a quorum check
	"leasenodrift",   // lease reads without a clock-drift bound
	"votenopersist",  // grant a vote before persisting votedFor
	"notruncate",     // skip truncating a conflicting log suffix
	"nodedup",        // apply a retried request twice
	"ackbeforefsync", // acknowledge before fsync
}
