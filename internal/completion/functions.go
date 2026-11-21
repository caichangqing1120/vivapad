package completion

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxCatalogBytes = 1 << 20
	maxFunctions    = 20000
	maxSuggestions  = 2000
	maxAPINameBytes = 255
)

// The callable names only, extracted from pinned Notepad++ v8.9.8.1 API data.
//
//go:embed api_go.txt
var goFunctions string

//go:embed api_python.txt
var pythonFunctions string

//go:embed api_signatures.json
var bundledSignatures []byte

var bundledFunctions = map[string]*FunctionCatalog{
	"go":     indexFunctions(strings.Fields(goFunctions), false, ""),
	"python": indexFunctions(strings.Fields(pythonFunctions), false, "."),
}

func init() {
	var data map[string]map[string][]Signature
	if err := json.Unmarshal(bundledSignatures, &data); err != nil {
		panic("invalid bundled API signatures: " + err.Error())
	}
	for language, catalog := range bundledFunctions {
		catalog.signatures = data[language]
	}
}

type functionCandidate struct{ key, name string }

type FunctionCatalog struct {
	entries    []functionCandidate
	ignoreCase bool
	additional string
	rustNames  bool
	signatures map[string][]Signature
	delimiters callDelimiters
}

func indexFunctions(names []string, ignoreCase bool, additional string) *FunctionCatalog {
	result := &FunctionCatalog{ignoreCase: ignoreCase, additional: additional}
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		key := result.key(name)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result.entries = append(result.entries, functionCandidate{key, name})
		}
	}
	sort.Slice(result.entries, func(i, j int) bool { return result.entries[i].key < result.entries[j].key })
	return result
}

// LoadFunctionCatalog accepts an optional per-language override in the private
// settings directory. A bad override is reported, rather than silently ignored.
func LoadFunctionCatalog(settingsDirectory, language string) (*FunctionCatalog, error) {
	language = strings.ToLower(language)
	for _, r := range language {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return nil, errors.New("invalid API language identifier")
		}
	}
	if settingsDirectory != "" && language != "" {
		path := filepath.Join(settingsDirectory, "apis", language+".xml")
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() || info.Size() > maxCatalogBytes {
				return nil, errors.New("API catalog must be a regular file at most 1 MiB")
			}
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			opened, err := file.Stat()
			if err != nil || !os.SameFile(info, opened) {
				return nil, errors.New("API catalog changed while opening")
			}
			return parseFunctionCatalog(file, language)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return bundledFunctions[language], nil
}

func ParseFunctionCatalog(reader io.Reader) (*FunctionCatalog, error) {
	return parseFunctionCatalog(reader, "")
}

func parseFunctionCatalog(reader io.Reader, language string) (*FunctionCatalog, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxCatalogBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCatalogBytes {
		return nil, errors.New("API catalog exceeds 1 MiB")
	}
	var root struct {
		XMLName xml.Name `xml:"NotepadPlus"`
		Blocks  []struct {
			Language    string `xml:"language,attr"`
			Environment struct {
				IgnoreCase string `xml:"ignoreCase,attr"`
				Additional string `xml:"additionalWordChar,attr"`
				Open       string `xml:"startFunc,attr"`
				Close      string `xml:"stopFunc,attr"`
				Separator  string `xml:"paramSeparator,attr"`
			} `xml:"Environment"`
			Keywords []struct {
				Name      string      `xml:"name,attr"`
				Func      string      `xml:"func,attr"`
				Overloads []Signature `xml:"Overload"`
			} `xml:"KeyWord"`
		} `xml:"AutoComplete"`
	}
	if err := decodeFunctionDocument(data, &root); err != nil {
		return nil, fmt.Errorf("invalid API catalog: %w", err)
	}
	if len(root.Blocks) != 1 || len(root.Blocks[0].Keywords) > maxFunctions {
		return nil, errors.New("API catalog must have one AutoComplete block with at most 20,000 entries")
	}
	block := root.Blocks[0]
	additional := block.Environment.Additional
	if strings.Trim(additional, ".$:-!?%@") != "" {
		return nil, errors.New("unsupported API identifier characters")
	}
	if language == "" {
		language = block.Language
	}
	rustNames := strings.EqualFold(language, "rust")
	tokens := FunctionCatalog{additional: additional, rustNames: rustNames}
	var names []string
	for _, keyword := range block.Keywords {
		if !strings.EqualFold(keyword.Func, "yes") {
			continue
		}
		if keyword.Name == "" || len(keyword.Name) > maxAPINameBytes || tokens.nameEnd(keyword.Name, 0, 0) != len(keyword.Name) {
			return nil, errors.New("invalid API function name")
		}
		names = append(names, keyword.Name)
	}
	// Notepad++ only disables case folding when the XML opts out.
	catalog := indexFunctions(names, !strings.EqualFold(block.Environment.IgnoreCase, "no"), additional)
	catalog.rustNames = rustNames
	catalog.delimiters = catalogDelimiters(block.Environment.Open, block.Environment.Close, block.Environment.Separator)
	catalog.signatures = make(map[string][]Signature)
	for _, keyword := range block.Keywords {
		if strings.EqualFold(keyword.Func, "yes") && len(keyword.Overloads) != 0 {
			key := catalog.key(keyword.Name)
			catalog.signatures[key] = append(catalog.signatures[key], keyword.Overloads...)
		}
	}
	return catalog, nil
}

func decodeFunctionDocument(data []byte, root any) error {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	decoder := xml.NewDecoder(bytes.NewReader(data))
	seenRoot := false
	for {
		start := decoder.InputOffset()
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) && seenRoot {
			return nil
		}
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if seenRoot {
				return errors.New("unexpected data after the root element")
			}
			if err := decoder.DecodeElement(root, &token); err != nil {
				return err
			}
			seenRoot = true
		case xml.CharData:
			// Outside the root, allow literal XML whitespace only. Token also
			// represents character references and CDATA as character data.
			if len(bytes.Trim(data[start:decoder.InputOffset()], " \t\r\n")) != 0 {
				return errors.New("unexpected text outside the root element")
			}
		case xml.Comment:
		case xml.ProcInst:
			if strings.EqualFold(token.Target, "xml") && (token.Target != "xml" || start != 0) {
				return errors.New("misplaced XML declaration")
			}
		case xml.Directive:
			if seenRoot {
				return errors.New("unexpected directive after the root element")
			}
		default:
			return errors.New("unexpected token outside the root element")
		}
	}
}

func (catalog *FunctionCatalog) key(value string) string {
	if catalog.ignoreCase {
		return strings.ToLower(value)
	}
	return value
}

func (catalog *FunctionCatalog) nameCharacter(r rune) bool {
	return wordCharacter(r) || strings.ContainsRune(catalog.additional, r)
}

func (catalog *FunctionCatalog) rustMacroSuffix(line string, bang int) bool {
	if !catalog.rustNames || bang <= 0 || line[bang] != '!' {
		return false
	}
	before, _ := utf8.DecodeLastRuneInString(line[:bang])
	after, _ := utf8.DecodeRuneInString(line[bang+1:])
	return wordCharacter(before) && after != '=' && after != '!' && !catalog.nameCharacter(after)
}

func (catalog *FunctionCatalog) nameStart(line string, caret int) int {
	start := caret
	if catalog.rustMacroSuffix(line, caret-1) {
		start--
	}
	for start > 0 && caret-start <= maxAPINameBytes {
		if catalog.rustNames && start >= 2 && line[start-2:start] == "::" {
			start -= 2
			continue
		}
		r, size := utf8.DecodeLastRuneInString(line[:start])
		if !catalog.nameCharacter(r) {
			break
		}
		start -= size
	}
	return start
}

func (catalog *FunctionCatalog) nameEnd(line string, start, caret int) int {
	end := caret
	for end < len(line) && end-start <= maxAPINameBytes {
		if catalog.rustNames && strings.HasPrefix(line[end:], "::") {
			end += 2
			continue
		}
		if end > start && catalog.rustMacroSuffix(line, end) {
			return end + 1
		}
		r, size := utf8.DecodeRuneInString(line[end:])
		if !catalog.nameCharacter(r) {
			break
		}
		end += size
	}
	return end
}

// Complete takes a byte caret on one logical line. Result offsets are rune
// positions relative to that line, matching the shared editor popup.
func (catalog *FunctionCatalog) Complete(line string, caret int) Result {
	if catalog == nil || caret <= 0 || caret > len(line) || caret < len(line) && !utf8.RuneStart(line[caret]) {
		return Result{}
	}
	start := catalog.nameStart(line, caret)
	if start == caret || caret-start > maxAPINameBytes {
		return Result{}
	}
	end := catalog.nameEnd(line, start, caret)
	if end-start > maxAPINameBytes {
		return Result{}
	}
	prefix, current := catalog.key(line[start:caret]), line[start:end]
	base := utf8.RuneCountInString(line[:start])
	result := Result{Start: base, End: base + utf8.RuneCountInString(current)}
	first := sort.Search(len(catalog.entries), func(i int) bool { return catalog.entries[i].key >= prefix })
	for _, entry := range catalog.entries[first:] {
		if !strings.HasPrefix(entry.key, prefix) {
			break
		}
		if entry.name == current {
			continue
		}
		if len(result.Candidates) == maxSuggestions {
			result.Truncated = true
			break
		}
		result.Candidates = append(result.Candidates, entry.name)
	}
	return result
}
