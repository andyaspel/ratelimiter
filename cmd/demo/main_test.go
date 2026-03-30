package main

import (
	"io"
	"log/slog"
	"testing"
)

func TestRunHelpCommandsReturnNil(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cases := [][]string{
		{"serve", "-h"},
		{"save", "-h"},
		{"list", "-h"},
	}

	for _, args := range cases {
		if err := run(args, logger); err != nil {
			t.Fatalf("expected %v to return nil on help, got %v", args, err)
		}
	}
}
