package printing

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestExpandBandRespectsOriginalWideBufferBeforePageSubstitution(t *testing.T) {
	text := strings.Repeat("界", 252) + "🙂a" + strings.Repeat("tail", 20)
	band := numberBand(prepareBand(TextBand{Left: text}, PageVariables{}), 3)
	if got := len(utf16.Encode([]rune(band.Left))); got != 255 || !strings.HasSuffix(band.Left, "🙂a") || strings.Contains(band.Left, "tail") {
		t.Fatalf("header length = %d UTF-16 units, suffix %q", got, band.Left[len(band.Left)-16:])
	}
	withPage := numberBand(prepareBand(TextBand{Left: strings.Repeat("x", 231) + "$(CURRENT_PRINTING_PAGE)"}, PageVariables{}), 12)
	if withPage.Left != strings.Repeat("x", 231)+"12" {
		t.Fatalf("page replacement ran before preprint truncation: %q", withPage.Left)
	}
}
