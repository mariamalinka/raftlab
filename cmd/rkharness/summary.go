package main

import (
	"encoding/json"
	"os"

	"raftkv/harness"
)

func writeSummary(path string, s harness.Summary) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
