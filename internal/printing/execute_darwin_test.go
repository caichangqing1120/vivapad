//go:build darwin

package printing

import (
	"errors"
	"os"
	"testing"

	"vivapad/internal/document"
)

func TestExecutePrintNowWithNoDefaultPrinterDoesNotSubmit(t *testing.T) {
	if os.Getenv("VIVAPAD_REQUIRE_NO_DEFAULT_PRINTER") != "1" {
		t.Skip("run only on a machine verified to have no default printer")
	}
	job, err := Prepare(document.New("new 1"), "", false)
	if err != nil {
		t.Fatal(err)
	}
	count, err := Execute(job, 13, 4, PageVariables{}, nil)
	if count != 0 || !errors.Is(err, ErrNoPrinter) {
		t.Fatalf("Print Now without default = %d pages, %v", count, err)
	}
}

func TestExecuteEmptyBufferNeverSubmitsToSelectedPrinter(t *testing.T) {
	name := os.Getenv("VIVAPAD_TEST_PRINT_METRICS")
	if name == "" {
		t.Skip("set VIVAPAD_TEST_PRINT_METRICS for a process-scoped printer")
	}
	t.Setenv("LPDEST", name)
	job, err := Prepare(document.New("new 1"), "", false)
	if err != nil {
		t.Fatal(err)
	}
	count, err := Execute(job, 13, 4, PageVariables{}, nil)
	if count != 0 || !errors.Is(err, ErrNoPages) {
		t.Fatalf("empty print = %d pages, %v", count, err)
	}
}
