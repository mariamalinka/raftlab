# Every figure in the report comes from one of these targets.
# Experiments need root (network namespaces, iptables) and Go 1.26+.
BIN ?= /tmp/raftkv-bin
OUT ?= results
export BIN OUT

.PHONY: build test g1 e1 e2 e3 e4 e5 e6 e7 e8 reproduce report clean

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

g1:
	bash scripts/g1.sh

e1 e2 e3 e4 e5 e6 e7 e8: build
	bash scripts/experiment.sh $(shell echo $@ | tr a-z A-Z)

reproduce: g1 e1 e2 e3 e4 e5 e6 e7 e8 report

report:
	python3 analysis/report.py $(OUT) > $(OUT)/REPORT.md

clean:
	rm -rf $(BIN)
