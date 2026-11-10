//go:build !darwin && !windows && !linux

package printing

import (
	"errors"
	"testing"
)

func TestUnavailableNativePrintNeverClaimsAJob(t *testing.T) {
	count, err := Execute(Job{Text: "do not spool"}, 13, 4, PageVariables{}, nil)
	if count != 0 || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported printer = %d pages, %v", count, err)
	}
}
