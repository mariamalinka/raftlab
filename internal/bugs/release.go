//go:build !bugs

package bugs

const (
	Figure8        = false
	LocalRead      = false
	LeaseNoDrift   = false
	VoteNoPersist  = false
	NoTruncate     = false
	NoDedup        = false
	AckBeforeFsync = false
)

// Compiled reports whether this binary was built with bug injection.
const Compiled = false

// Active returns the names of the bugs switched on in this process.
func Active() []string { return nil }
