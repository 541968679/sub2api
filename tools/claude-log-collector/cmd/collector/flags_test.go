package main

import "testing"

func TestParseArgs(t *testing.T) {
	opts, err := parseArgs([]string{
		"--out", `D:\tmp\out`,
		"--include-sessions",
		"--all-logs",
		"--vault", `D:\vault`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.OutDir != `D:\tmp\out` || !opts.IncludeSessions || !opts.AllLogs || opts.ExtraVault != `D:\vault` {
		t.Fatalf("%+v", opts)
	}
	if _, err := parseArgs([]string{"--nope"}); err == nil {
		t.Fatal("expected unknown flag error")
	}
}
