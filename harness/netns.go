// Package harness is the Jepsen-style fault-injection harness: it runs a
// cluster (raftkv or etcd) with one Linux network namespace per node, drives
// a register workload against it while a nemesis injects faults, and checks
// the recorded history for linearizability with Porcupine.
//
// Each node gets its own namespace, veth pair and IP on a bridge, so faults
// are real: kill -9, SIGSTOP/SIGCONT and iptables DROP rules. Clients run in
// the host namespace and are never partitioned (as in Jepsen).
package harness

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	bridge   = "rkbr0"
	hostIP   = "10.77.0.1"
	subnet   = "/24"
	nsPrefix = "rkn"
)

func nodeIP(i int) string   { return fmt.Sprintf("10.77.0.%d", 11+i) }
func nsName(i int) string   { return fmt.Sprintf("%s%d", nsPrefix, i+1) }
func vethName(i int) string { return fmt.Sprintf("rkv%d", i+1) }

func run(args ...string) error {
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func inNS(i int, args ...string) error {
	return run(append([]string{"ip", "netns", "exec", nsName(i)}, args...)...)
}

// Network is the bridge plus n node namespaces.
type Network struct{ N int }

func SetupNetwork(n int) (*Network, error) {
	TeardownNetwork(n)
	steps := [][]string{
		{"ip", "link", "add", bridge, "type", "bridge"},
		{"ip", "addr", "add", hostIP + subnet, "dev", bridge},
		{"ip", "link", "set", bridge, "up"},
	}
	for i := range n {
		steps = append(steps,
			[]string{"ip", "netns", "add", nsName(i)},
			[]string{"ip", "link", "add", vethName(i), "type", "veth", "peer", "name", "eth0", "netns", nsName(i)},
			[]string{"ip", "link", "set", vethName(i), "master", bridge},
			[]string{"ip", "link", "set", vethName(i), "up"},
			[]string{"ip", "netns", "exec", nsName(i), "ip", "addr", "add", nodeIP(i) + subnet, "dev", "eth0"},
			[]string{"ip", "netns", "exec", nsName(i), "ip", "link", "set", "eth0", "up"},
			[]string{"ip", "netns", "exec", nsName(i), "ip", "link", "set", "lo", "up"},
		)
	}
	for _, s := range steps {
		if err := run(s...); err != nil {
			return nil, err
		}
	}
	return &Network{N: n}, nil
}

func TeardownNetwork(n int) {
	for i := range max(n, 9) {
		exec.Command("ip", "netns", "del", nsName(i)).Run()
		exec.Command("ip", "link", "del", vethName(i)).Run()
	}
	exec.Command("ip", "link", "del", bridge).Run()
}

// Partition installs DROP rules so that nodes in different groups cannot
// talk. Nodes not listed in any group keep full connectivity. Each node drops
// inbound packets from the nodes it must not hear, so the cut is symmetric.
func (nw *Network) Partition(groups [][]int) error {
	if err := nw.Heal(); err != nil {
		return err
	}
	group := map[int]int{}
	for g, ns := range groups {
		for _, i := range ns {
			group[i] = g
		}
	}
	for i := range nw.N {
		gi, ok := group[i]
		if !ok {
			continue
		}
		for j := range nw.N {
			if gj, ok := group[j]; ok && gj != gi {
				if err := inNS(i, "iptables", "-A", "INPUT", "-s", nodeIP(j), "-j", "DROP"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Cut drops traffic between specific pairs (for bridge partitions).
func (nw *Network) Cut(pairs [][2]int) error {
	for _, p := range pairs {
		if err := inNS(p[0], "iptables", "-A", "INPUT", "-s", nodeIP(p[1]), "-j", "DROP"); err != nil {
			return err
		}
		if err := inNS(p[1], "iptables", "-A", "INPUT", "-s", nodeIP(p[0]), "-j", "DROP"); err != nil {
			return err
		}
	}
	return nil
}

func (nw *Network) Heal() error {
	for i := range nw.N {
		if err := inNS(i, "iptables", "-F", "INPUT"); err != nil {
			return err
		}
	}
	return nil
}
