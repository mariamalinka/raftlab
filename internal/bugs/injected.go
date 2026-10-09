//go:build bugs

package bugs

import (
	"fmt"
	"os"
	"strings"
)

var (
	Figure8        bool
	LocalRead      bool
	LeaseNoDrift   bool
	VoteNoPersist  bool
	NoTruncate     bool
	NoDedup        bool
	AckBeforeFsync bool
)

const Compiled = true

var active []string

func init() {
	for _, name := range strings.Split(os.Getenv("RAFTKV_BUGS"), ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		switch name {
		case "figure8":
			Figure8 = true
		case "localread":
			LocalRead = true
		case "leasenodrift":
			LeaseNoDrift = true
		case "votenopersist":
			VoteNoPersist = true
		case "notruncate":
			NoTruncate = true
		case "nodedup":
			NoDedup = true
		case "ackbeforefsync":
			AckBeforeFsync = true
		default:
			fmt.Fprintf(os.Stderr, "bugs: unknown bug %q (known: %v)\n", name, Names)
			os.Exit(2)
		}
		active = append(active, name)
	}
}

// Active returns the names of the bugs switched on in this process.
func Active() []string { return active }
