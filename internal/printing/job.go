package printing

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

var ErrNoPrinter = errors.New("printing: no default printer")
var ErrNoPages = errors.New("printing: no printable pages")
var ErrUnsupported = errors.New("printing: native printing is unsupported")
var ErrPrintCanceled = errors.New("printing: print dialog canceled")

type Scope uint8

const (
	ScopeAll Scope = iota
	ScopeSelection
)

type ColorMode uint8

const (
	ColorOnWhite ColorMode = iota + 1
	ColorWYSIWYG
	ColorInvertLight
	ColorBlackOnWhite
)

type Margins struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

type TextBand struct {
	Left   string `json:"left"`
	Middle string `json:"middle"`
	Right  string `json:"right"`
}

type BandFont struct {
	Size   int  `json:"size"`
	Bold   bool `json:"bold"`
	Italic bool `json:"italic"`
}

func (font BandFont) Points() int {
	if font.Size == 0 {
		return 9
	}
	return font.Size
}

type Settings struct {
	LineNumbers       bool      `json:"line_numbers"`
	ColorMode         ColorMode `json:"color_mode"`
	FormFeedPageBreak bool      `json:"form_feed_page_break"`
	MarginMM          Margins   `json:"margin_mm"`
	Header            TextBand  `json:"header"`
	Footer            TextBand  `json:"footer"`
	HeaderFont        BandFont  `json:"header_font"`
	FooterFont        BandFont  `json:"footer_font"`
}

func DefaultSettings() Settings {
	return Settings{LineNumbers: true, ColorMode: ColorOnWhite}
}

func (settings Settings) Validate() error {
	if settings.ColorMode < ColorOnWhite || settings.ColorMode > ColorBlackOnWhite {
		return fmt.Errorf("printing: invalid color mode")
	}
	for _, value := range []int{settings.MarginMM.Left, settings.MarginMM.Top, settings.MarginMM.Right, settings.MarginMM.Bottom} {
		if value < 0 || value > 10000 {
			return fmt.Errorf("printing: invalid margin")
		}
	}
	for _, font := range []BandFont{settings.HeaderFont, settings.FooterFont} {
		if font.Size != 0 && (font.Size < 6 || font.Size > 14) {
			return fmt.Errorf("printing: invalid header or footer font size")
		}
	}
	for _, value := range []string{settings.Header.Left, settings.Header.Middle, settings.Header.Right,
		settings.Footer.Left, settings.Footer.Middle, settings.Footer.Right} {
		if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 || len(utf16.Encode([]rune(value))) > 255 {
			return fmt.Errorf("printing: invalid header or footer text")
		}
		for _, character := range value {
			if character < 0x20 || character == 0x7f {
				return fmt.Errorf("printing: invalid header or footer text")
			}
		}
	}
	return nil
}

type Job struct {
	Title       string
	Text        string
	Language    string
	Interactive bool
	Scope       Scope
	Selection   text.Span
	Settings    Settings
}

type PrintChoice struct {
	Scope               Scope
	FirstPage, LastPage int
	Copies              int
	Ranges              []PageRange
}

type PageRange struct {
	First, Last int
}

// GTK's open-ended print range resolves against the paginated document.
const pageRangeThroughEnd = -1

func Prepare(doc *document.Document, language string, interactive bool) (Job, error) {
	if doc == nil {
		return Job{}, fmt.Errorf("printing: no document")
	}
	job := Job{
		Title: doc.Path, Text: doc.Editor.String(), Language: language,
		Interactive: interactive,
		Settings:    DefaultSettings(),
	}
	if job.Title == "" {
		job.Title = doc.Name
	}
	if interactive {
		selection := doc.Editor.Selection()
		if selection.Start != selection.End {
			job.Scope = ScopeSelection
			job.Selection = text.Span{
				Start: doc.Editor.ByteOffset(selection.Start),
				End:   doc.Editor.ByteOffset(selection.End),
			}
		}
	}
	return job, nil
}
