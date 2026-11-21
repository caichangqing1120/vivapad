package session

import (
	"io"
	"strings"
	"testing"
)

func TestSessionDecodersRequireActualEOFAtSizeLimit(t *testing.T) {
	const limit = 1 << 20
	for _, tc := range []struct {
		name, prefix string
		decode       func(io.Reader) error
	}{
		{"JSON", `{"version":12,"paths":[],"active":""}`, func(r io.Reader) error { _, err := decodeSession(r); return err }},
		{"XML", `<NotepadPlus><Session/></NotepadPlus>`, func(r io.Reader) error { _, err := decodeNPPSession(r); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			full := tc.prefix + strings.Repeat(" ", limit-len(tc.prefix))
			if err := tc.decode(strings.NewReader(full)); err != nil {
				t.Fatalf("exactly 1 MiB: %v", err)
			}
			if err := tc.decode(strings.NewReader(full + " ")); err == nil || !strings.Contains(err.Error(), "exceeds 1 MiB") {
				t.Fatalf("1 MiB plus one byte: %v, want a size error", err)
			}
		})
	}
}

func TestNPPSessionRejectsTextOutsideRoot(t *testing.T) {
	const root = `<NotepadPlus><Session/></NotepadPlus>`
	for _, source := range []string{"invalid " + root, root + " invalid", root + "\u00a0", root + "<extra/>"} {
		if _, err := decodeNPPSession(strings.NewReader(source)); err == nil {
			t.Errorf("accepted malformed XML %q", source)
		}
	}
	const valid = "\ufeff<?xml version=\"1.0\" encoding=\"UTF-8\"?><!--header-->\n" + root + "\n<!--footer--><?app keep?>\t"
	if _, err := decodeNPPSession(strings.NewReader(valid)); err != nil {
		t.Fatalf("valid XML with a BOM, comments, and trailing whitespace: %v", err)
	}
}
