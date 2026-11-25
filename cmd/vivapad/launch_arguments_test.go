package main

import (
	"slices"
	"testing"
)

func TestParseLaunchArgumentsPreservesFileOperands(t *testing.T) {
	for _, test := range []struct {
		name  string
		args  []string
		paths []string
		help  bool
	}{
		{name: "session"},
		{name: "help", args: []string{"--help"}, help: true},
		{name: "short help", args: []string{"-h", "ignored.txt"}, help: true},
		{name: "literal help filename", args: []string{"--", "--help", "file with spaces.txt"}, paths: []string{"--help", "file with spaces.txt"}},
		{name: "only terminator", args: []string{"--"}},
		{name: "positional files", args: []string{"-draft.txt", "--help", "--"}, paths: []string{"-draft.txt", "--help", "--"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			paths, help := parseLaunchArguments(test.args)
			if help != test.help || !slices.Equal(paths, test.paths) {
				t.Fatalf("args %q mapped to paths=%q help=%t; want %q, %t", test.args, paths, help, test.paths, test.help)
			}
		})
	}
}
