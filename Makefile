# Every figure in the report comes from one of these targets.
# Experiments need root (network namespaces, iptables) and Go 1.26+.
BIN ?= /tmp/raftkv-bin
OUT ?= results
export BIN OUT

.PHONY: bugtests build test g1 e1 e2 e3 e4 e5 e6 e7 e8 reproduce report clean

build:
	mkdir -p $(BIN)
	go build -o $(BIN)/raftkv ./cmd/raftkv
	go build -tags bugs -o $(BIN)/raftkv-bugs ./cmd/raftkv
	go build -o $(BIN)/rkharness ./cmd/rkharness
	go build -o $(BIN)/loadgen ./cmd/loadgen
	cd tools/etcd && go build -o $(BIN)/etcd .   # etcd v3.7.2, pinned in tools/etcd/go.mod

test:
	go vet ./...
	go test -race ./...

# Each injected bug must make its deterministic test fail (RQ3 reachability).
bugtests:
	@for b in votenopersist:raft:VoteNoPersist notruncate:raft:NoTruncate figure8:raft:Figure8 nodedup:kv:NoDedup \
	         ackbeforefsync:raft:AckBeforeFsync leasenodrift:raft:LeaseNoDrift localread:kv:LocalRead; do \
	  n=$${b%%:*}; r=$${b#*:}; p=$${r%%:*}; t=$${r#*:}; \
	  if RAFTKV_BUGS=$$n go test -tags bugs -count=1 -run "^TestBug$$t$$" ./internal/$$p 2>&1 | grep -q -- "^--- FAIL: TestBug$$t "; then \
	    echo "$$n: caught"; else echo "$$n: NOT caught"; exit 1; fi; \
	done

g1:
	bash scripts/g1.sh

e1 e2 e3 e4 e5 e6 e7 e8: build
	bash scripts/experiment.sh $(shell echo $@ | tr a-z A-Z)

reproduce: g1 e1 e2 e3 e4 e5 e6 e7 e8 report

report:
	python3 analysis/report.py $(OUT) > $(OUT)/REPORT.md

clean:
	rm -rf $(BIN)
