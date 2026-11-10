//go:build linux

package printing

import "testing"

func TestClosedLinuxPrinterCannotSubmit(t *testing.T) {
	printer := &Printer{cups: true, choice: PrintChoice{Scope: ScopeAll}}
	printer.Close()
	if printer.cups {
		t.Fatal("closed CUPS printer still owns a submission path")
	}
	if err := printer.PrintPages(Job{Scope: ScopeAll}, []Page{{Number: 1}}); err == nil {
		t.Fatal("closed CUPS printer could submit another job")
	}
}
