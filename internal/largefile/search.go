package largefile

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxMatches = 2000

type SearchOptions struct {
	MatchCase, WholeWord, Regex bool
}

type Match struct {
	Line, Column int
	Offset, End  int64
	Preview      string
}

type SearchResult struct {
	Matches   []Match
	Truncated bool
}

func (file *File) Search(ctx context.Context, query string, options SearchOptions, limit int) (SearchResult, error) {
	if limit <= 0 || limit > MaxMatches {
		limit = MaxMatches
	}
	var result SearchResult
	err := file.scanMatches(ctx, query, options, func(hit Match) bool {
		if len(result.Matches) == limit {
			result.Truncated = true
			return false
		}
		result.Matches = append(result.Matches, hit)
		return true
	})
	return result, err
}

func (file *File) Find(ctx context.Context, query string, options SearchOptions, boundary int64, backward bool) (hit Match, found bool, err error) {
	var wrap Match
	err = file.scanMatches(ctx, query, options, func(candidate Match) bool {
		if backward {
			wrap = candidate
			if candidate.Offset >= boundary && found {
				return false
			}
			if candidate.Offset < boundary {
				hit, found = candidate, true
			}
		} else {
			if wrap.Line == 0 {
				wrap = candidate
			}
			if candidate.Offset >= boundary {
				hit, found = candidate, true
				return false
			}
		}
		return true
	})
	if !found && wrap.Line != 0 {
		hit, found = wrap, true
	}
	return
}

func (file *File) scanMatches(ctx context.Context, query string, options SearchOptions, visit func(Match) bool) error {
	if query == "" {
		return fmt.Errorf("enter text to find")
	}
	if len(query) > pageSize {
		return fmt.Errorf("search pattern is too long")
	}
	if !file.begin() {
		return file.ctx.Err()
	}
	defer file.work.Done()
	check := func() error {
		if err := file.ctx.Err(); err != nil {
			return err
		}
		return ctx.Err()
	}
	if err := check(); err != nil {
		return err
	}
	pattern := query
	if !options.Regex {
		pattern = regexp.QuoteMeta(query)
	}
	if !options.MatchCase {
		pattern = "(?i:" + pattern + ")"
	}
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	reader := &searchReader{check: check, source: file.handle, size: file.Progress().Size, buffer: make([]byte, pageSize), base: -1}
	var scratch []byte
	var afterPrefix *regexp.Regexp
	for line, offset := 1, int64(0); ; line++ {
		end, next, err := lineBounds(reader.page, offset, reader.size)
		if err != nil {
			return err
		}
		length := end - offset
		if length <= pageSize {
			var value []byte
			if length > 0 {
				chunk, err := reader.page(offset)
				if err != nil {
					return err
				}
				if length <= int64(len(chunk)) {
					value = chunk[:length]
				} else {
					if cap(scratch) < int(length) {
						scratch = make([]byte, pageSize)
					}
					value = scratch[:length]
					if _, err := reader.ReadAt(value, offset); err != nil {
						return err
					}
				}
			}
			last, column := 0, 1
			for _, span := range expression.FindAllIndex(value, -1) {
				if err := check(); err != nil {
					return err
				}
				start, stop := span[0], span[1]
				if options.WholeWord {
					before, _ := utf8.DecodeLastRune(value[:start])
					after, _ := utf8.DecodeRune(value[stop:])
					if wordRune(before) || wordRune(after) {
						continue
					}
				}
				column += utf8.RuneCount(value[last:start])
				last = start
				preview := previewAround(value, start)
				if !visit(Match{line, column, offset + int64(start), offset + int64(stop), preview}) {
					return check()
				}
			}
		} else {
			if afterPrefix == nil {
				afterPrefix, err = regexp.Compile(`(?s:\A..*?)(` + pattern + `)`)
				if err != nil {
					return err
				}
			}
			proceed, err := scanLongLine(reader, line, offset, end, expression, afterPrefix, options.WholeWord, visit)
			if err != nil || !proceed {
				return err
			}
		}
		if err := check(); err != nil {
			return err
		}
		if end == reader.size {
			return nil
		}
		offset = next
	}
}

func wordRune(value rune) bool {
	return value == '_' || unicode.IsLetter(value) || unicode.IsNumber(value) || unicode.IsMark(value)
}

func previewAround(value []byte, at int) string {
	start := max(0, at-60)
	end := min(len(value), start+200)
	for start < at && !utf8.RuneStart(value[start]) {
		start++
	}
	for end > start && end < len(value) && !utf8.RuneStart(value[end]) {
		end--
	}
	return strings.ToValidUTF8(string(value[start:end]), "\uFFFD")
}

type searchReader struct {
	source io.ReaderAt
	check  func() error
	buffer []byte
	base   int64
	size   int64
	length int
}

func (reader *searchReader) ReadAt(data []byte, offset int64) (int, error) {
	if err := reader.check(); err != nil {
		return 0, err
	}
	return reader.source.ReadAt(data, offset)
}

func (reader *searchReader) page(offset int64) ([]byte, error) {
	if err := reader.check(); err != nil {
		return nil, err
	}
	base := offset / pageSize * pageSize
	if base != reader.base {
		length := int(min(int64(pageSize), reader.size-base))
		count, err := reader.ReadAt(reader.buffer[:length], base)
		if err != nil {
			return nil, err
		}
		reader.length, reader.base = count, base
	}
	return reader.buffer[int(offset-base):reader.length], nil
}

func scanLongLine(reader *searchReader, line int, start, end int64, expression, afterPrefix *regexp.Regexp, wholeWord bool, visit func(Match) bool) (bool, error) {
	var section io.SectionReader
	input := bufio.NewReaderSize(&section, 32<<10)
	last, column, previousEnd := start, 1, int64(-1)
	for cursor := start; cursor <= end; {
		base := cursor
		matcher := expression
		if cursor > start {
			_, width, err := runeBefore(reader, start, cursor)
			if err != nil {
				return false, err
			}
			base -= int64(width)
			matcher = afterPrefix
		}
		section = *io.NewSectionReader(reader, base, end-base)
		input.Reset(&section)
		span := matcher.FindReaderSubmatchIndex(input)
		if err := reader.check(); err != nil {
			return false, err
		}
		if len(span) < 2 {
			return true, nil
		}
		if matcher == afterPrefix {
			span = span[2:4]
		}
		found, stop := base+int64(span[0]), base+int64(span[1])
		accept := found != stop || found != previousEnd
		previousEnd = stop
		cursor = stop
		if found == stop {
			_, width, err := runeAfter(reader, stop, end)
			if err != nil {
				return false, err
			}
			cursor += int64(max(1, width))
		}
		if wholeWord {
			before, _, err := runeBefore(reader, start, found)
			if err != nil {
				return false, err
			}
			after, _, err := runeAfter(reader, stop, end)
			if err != nil {
				return false, err
			}
			accept = accept && !wordRune(before) && !wordRune(after)
		}
		if accept {
			section = *io.NewSectionReader(reader, last, found-last)
			input.Reset(&section)
			for last < found {
				_, width, err := input.ReadRune()
				if err != nil {
					return false, err
				}
				column++
				last += int64(width)
			}
			window := max(start, found-60)
			var preview [200]byte
			value := preview[:min(int64(len(preview)), end-window)]
			if len(value) != 0 {
				if _, err := reader.ReadAt(value, window); err != nil {
					return false, err
				}
			}
			if !visit(Match{line, column, found, stop, previewAround(value, int(found-window))}) {
				return false, reader.check()
			}
		}
	}
	return true, nil
}

func runeBefore(reader *searchReader, start, offset int64) (rune, int, error) {
	var buffer [utf8.UTFMax]byte
	length := min(int64(len(buffer)), offset-start)
	if length == 0 {
		return utf8.RuneError, 0, nil
	}
	if _, err := reader.ReadAt(buffer[:length], offset-length); err != nil {
		return 0, 0, err
	}
	value, width := utf8.DecodeLastRune(buffer[:length])
	return value, width, nil
}

func runeAfter(reader *searchReader, offset, end int64) (rune, int, error) {
	var buffer [utf8.UTFMax]byte
	length := min(int64(len(buffer)), end-offset)
	if length == 0 {
		return utf8.RuneError, 0, nil
	}
	if _, err := reader.ReadAt(buffer[:length], offset); err != nil {
		return 0, 0, err
	}
	value, width := utf8.DecodeRune(buffer[:length])
	return value, width, nil
}
