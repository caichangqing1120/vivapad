package document

import (
	"math/rand/v2"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestReplaceAllReportsOriginalByteEditsWithRegexCaptures(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("猫1\n猫2")
	count, edits, err := doc.ReplaceAllWithEdits(`(猫)(\d)`, "$2-$1\n", SearchOptions{Regex: true})
	if err != nil || count != 2 || len(edits) != 2 ||
		edits[0] != (ReplacementEdit{Start: 0, End: 4, Text: "1-猫\n"}) ||
		edits[1] != (ReplacementEdit{Start: 5, End: 9, Text: "2-猫\n"}) ||
		doc.Editor.String() != "1-猫\n\n2-猫\n" {
		t.Fatalf("regex edits = %+v, count=%d, text=%q, err=%v", edits, count, doc.Editor.String(), err)
	}
}

func TestFindOptionsRespectCaseAndWholeWords(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("cat scatter CAT cat_ cat")
	doc.Editor.MoveTo(0, false)
	options := SearchOptions{MatchCase: true, WholeWord: true}
	if found, err := doc.FindWith("cat", options, false); err != nil || !found || doc.Editor.Selection().Start != 0 {
		t.Fatalf("first match: found=%v err=%v selection=%+v", found, err, doc.Editor.Selection())
	}
	if found, err := doc.FindWith("cat", options, false); err != nil || !found || doc.Editor.Selection().Start != 21 {
		t.Fatalf("next whole word: found=%v err=%v selection=%+v", found, err, doc.Editor.Selection())
	}
	doc.Editor.MoveTo(0, false)
	if found, err := doc.FindWith("cat", SearchOptions{WholeWord: true}, true); err != nil || !found || doc.Editor.Selection().Start != 21 {
		t.Fatalf("wrapped previous: found=%v err=%v selection=%+v", found, err, doc.Editor.Selection())
	}
}

func TestUnicodeWholeWordDoesNotMatchInsideIdentifier(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("猫 猫咪 猫")
	doc.Editor.MoveTo(0, false)
	options := SearchOptions{WholeWord: true}
	if found, err := doc.FindWith("猫", options, false); err != nil || !found {
		t.Fatalf("first match: found=%v err=%v", found, err)
	}
	if found, err := doc.FindWith("猫", options, false); err != nil || !found || doc.Editor.Selection().Start != 5 {
		t.Fatalf("last match: found=%v err=%v selection=%+v", found, err, doc.Editor.Selection())
	}
}

func TestWholeWordSearchUsesUnicodeCharacterEdges(t *testing.T) {
	for _, tc := range []struct {
		name, value, query, replaced string
		want                         []text.Span
	}{
		{"combining marks", "e\u0301 e e\u0301 e", "e", "e\u0301 ! e\u0301 !", []text.Span{{Start: 4, End: 5}, {Start: 10, End: 11}}},
		{"other Unicode numbers", "x\u00b2 x x\u2160 x", "x", "x\u00b2 ! x\u2160 !", []text.Span{{Start: 4, End: 5}, {Start: 11, End: 12}}},
		{"punctuation runs", ":: a:b : ", ":", ":: a!b ! ", []text.Span{{Start: 4, End: 5}, {Start: 7, End: 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name  string
				regex bool
			}{{"literal", false}, {"regex", true}} {
				t.Run(mode.name, func(t *testing.T) {
					doc := New("fixture")
					doc.Editor.Insert(tc.value)
					options := SearchOptions{MatchCase: true, WholeWord: true, Regex: mode.regex}
					if matches, err := doc.MatchesWith(tc.query, options); err != nil || !slices.Equal(matches, tc.want) {
						t.Errorf("all matches = %v, %v; want %v", matches, err, tc.want)
					}
					if matches, truncated, err := doc.MatchesWithLimit(tc.query, options, 1); err != nil || !truncated || !slices.Equal(matches, tc.want[:1]) {
						t.Errorf("limited matches = %v, truncated %t, %v; want %v and truncation", matches, truncated, err, tc.want[:1])
					}
					for _, backward := range []bool{false, true} {
						doc.Editor.MoveTo(0, false)
						want := tc.want[0]
						if backward {
							want = tc.want[1]
						}
						found, err := doc.FindWith(tc.query, options, backward)
						selection := doc.Editor.Selection()
						got := text.Span{Start: doc.Editor.ByteOffset(selection.Start), End: doc.Editor.ByteOffset(selection.End)}
						if err != nil || !found || got != want {
							t.Errorf("find (backward %t) = %v, %t, %v; want %v", backward, got, found, err, want)
						}
					}
					doc.Editor.SetSelection(text.Range{Start: 0, End: 1}) // The first occurrence is only part of a word or punctuation run.
					if changed, _, err := doc.ReplaceCurrentWithEdit(tc.query, "!", options); err != nil || changed || doc.Editor.String() != tc.value {
						t.Fatalf("replace accepted a partial match: changed %t, text %q, err %v", changed, doc.Editor.String(), err)
					}
					if count, _, err := doc.ReplaceAllWithEdits(tc.query, "!", options); err != nil || count != len(tc.want) || doc.Editor.String() != tc.replaced {
						t.Errorf("replace all count %d, text %q, err %v; want %d, %q", count, doc.Editor.String(), err, len(tc.want), tc.replaced)
					}
				})
			}
		})
	}
}

func TestLiteralFindKeepsMatchOrderAndDirectionalCaret(t *testing.T) {
	for _, tc := range []struct {
		name, value, query string
		options            SearchOptions
		caret              int
		backward           bool
		want               text.Range
	}{
		{"forward across overlapping candidate", "aaaaa", "aa", SearchOptions{MatchCase: true}, 1, false, text.Range{Start: 1, End: 3}},
		{"previous before partial match", "aaaaa", "aa", SearchOptions{}, 3, true, text.Range{Start: 1, End: 3}},
		{"wrap backward", "aaaaa", "aa", SearchOptions{}, 0, true, text.Range{Start: 3, End: 5}},
		{"wrap forward", "aaaaa", "aa", SearchOptions{MatchCase: true}, 5, false, text.Range{Start: 0, End: 2}},
		{"folded overlap at caret", "kKkkk", "kk", SearchOptions{}, 1, false, text.Range{Start: 1, End: 3}},
		{"whole word checks before forward caret", "xcat cat", "cat", SearchOptions{WholeWord: true}, 1, false, text.Range{Start: 5, End: 8}},
		{"whole word checks after backward caret", "cat catx", "cat", SearchOptions{MatchCase: true, WholeWord: true}, 7, true, text.Range{Start: 0, End: 3}},
		{"Unicode folding with different byte widths", "xK K", "k", SearchOptions{WholeWord: true}, 0, false, text.Range{Start: 3, End: 4}},
		{"Unicode folding after wrap", "K K", "k", SearchOptions{WholeWord: true}, 3, false, text.Range{Start: 0, End: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := New("new 1")
			doc.Editor.Insert(tc.value)
			doc.Editor.MoveTo(tc.caret, false)
			found, err := doc.FindWith(tc.query, tc.options, tc.backward)
			wantCaret := tc.want.End
			if tc.backward {
				wantCaret = tc.want.Start
			}
			if !found || err != nil || doc.Editor.Selection() != tc.want || doc.Editor.Caret() != wantCaret {
				t.Fatalf("found=%v, err=%v, selection=%+v, caret=%d; want %+v, caret=%d", found, err, doc.Editor.Selection(), doc.Editor.Caret(), tc.want, wantCaret)
			}
		})
	}
}

func TestWholeWordLiteralSearchReconsidersRejectedOverlap(t *testing.T) {
	for _, tc := range []struct {
		name, value, query, replaced, single string
		matchCase                            bool
		want                                 [2]text.Span
	}{
		{"case sensitive", "xa.a.a a.a.a", "a.a", "xa.! !.a", "xa.! a.a.a", true, [2]text.Span{{Start: 3, End: 6}, {Start: 7, End: 10}}},
		{"folded UTF-8", "x\u212a.k.k K.K", "k.k", "x\u212a.! !", "x\u212a.! K.K", false, [2]text.Span{{Start: 5, End: 8}, {Start: 9, End: 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := New("fixture")
			doc.Editor.Insert(tc.value)
			options := SearchOptions{MatchCase: tc.matchCase, WholeWord: true}
			if matches, err := doc.MatchesWith(tc.query, options); err != nil || !slices.Equal(matches, tc.want[:]) {
				t.Errorf("all matches = %v, %v; want %v", matches, err, tc.want)
			}
			for _, limit := range []int{0, 1, 2} {
				if matches, truncated, err := doc.MatchesWithLimit(tc.query, options, limit); err != nil || truncated != (limit < len(tc.want)) || !slices.Equal(matches, tc.want[:limit]) {
					t.Errorf("limit %d: matches %v, truncated %t, %v; want %v", limit, matches, truncated, err, tc.want[:limit])
				}
			}
			for _, backward := range []bool{false, true} {
				// Search backwards just after the first good hit, before the second.
				start := 0
				if backward {
					start = doc.Editor.RuneIndex(tc.want[0].End)
				}
				doc.Editor.MoveTo(start, false)
				found, err := doc.FindWith(tc.query, options, backward)
				selection := doc.Editor.Selection()
				got := text.Span{Start: doc.Editor.ByteOffset(selection.Start), End: doc.Editor.ByteOffset(selection.End)}
				if err != nil || !found || got != tc.want[0] {
					t.Errorf("find (backward %t) = %v, %t, %v; want %v", backward, got, found, err, tc.want[0])
				}
			}
			one := New("single replacement")
			one.Editor.Insert(tc.value)
			one.Editor.SetSelection(text.Range{Start: one.Editor.RuneIndex(tc.want[0].Start), End: one.Editor.RuneIndex(tc.want[0].End)})
			if changed, _, err := one.ReplaceCurrentWithEdit(tc.query, "!", options); err != nil || !changed || one.Editor.String() != tc.single {
				t.Errorf("replace current = %t, %q, %v; want %q", changed, one.Editor.String(), err, tc.single)
			}
			if count, _, err := doc.ReplaceAllWithEdits(tc.query, "!", options); err != nil || count != 2 || doc.Editor.String() != tc.replaced {
				t.Errorf("replace all = %d, %q, %v; want 2, %q", count, doc.Editor.String(), err, tc.replaced)
			}
		})
	}
}

func TestLiteralFindAllocationsStaySmallOnDenseMatches(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert(strings.Repeat("cat ", 50_000))
	for _, tc := range []struct {
		name     string
		options  SearchOptions
		caret    int
		backward bool
		want     int
	}{
		{"default next near the start", SearchOptions{}, 0, false, 0},
		{"default previous near the end", SearchOptions{}, doc.Editor.RuneCount(), true, doc.Editor.RuneCount() - 4},
		{"case-sensitive previous near the end", SearchOptions{MatchCase: true}, doc.Editor.RuneCount(), true, doc.Editor.RuneCount() - 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allocations := testing.AllocsPerRun(3, func() {
				doc.Editor.MoveTo(tc.caret, false)
				if found, err := doc.FindWith("cat", tc.options, tc.backward); !found || err != nil || doc.Editor.Selection().Start != tc.want {
					t.Fatalf("found=%v, err=%v, selection=%+v", found, err, doc.Editor.Selection())
				}
			})
			t.Logf("allocations per find: %.0f", allocations)
			if allocations > 128 {
				t.Fatalf("one literal find allocated %.0f times for 50,000 matches", allocations)
			}
		})
	}
}

func TestLiteralMatchStreamAgreesWithQuotedRegexp(t *testing.T) {
	random := rand.New(rand.NewPCG(19, 31))
	alphabet := []rune("aAbBkkKSsſΣσς猫.\n")
	for trial := 0; trial < 1500; trial++ {
		characters := make([]rune, 1+random.IntN(64))
		for index := range characters {
			characters[index] = alphabet[random.IntN(len(alphabet))]
		}
		from := random.IntN(len(characters))
		to := from + 1 + random.IntN(min(len(characters)-from, 8))
		value, query := string(characters), string(characters[from:to])
		for _, matchCase := range []bool{false, true} {
			expression, err := compileSearch(query, SearchOptions{MatchCase: matchCase})
			if err != nil {
				t.Fatal(err)
			}
			want := expression.FindAllStringIndex(value, -1)
			var got [][]int
			visitLiteralMatches(value, query, SearchOptions{MatchCase: matchCase}, func(match text.Span) bool {
				got = append(got, []int{match.Start, match.End})
				return true
			})
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("case=%v, Find %q in %q: ranges %v, want %v", matchCase, query, value, got, want)
			}
		}
	}
}

func TestReplaceSelectedLiteralUsesMatchBoundaries(t *testing.T) {
	doc := New("fixture")
	doc.Editor.Insert("x \u212A tail")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 3, End: 2})
	changed, edit, err := doc.ReplaceCurrentWithEdit("k", "$1", SearchOptions{WholeWord: true})
	if err != nil || !changed || edit != (ReplacementEdit{Start: 2, End: 5, Text: "$1"}) || doc.Editor.String() != "x $1 tail" {
		t.Fatalf("replacement: changed=%v edit=%+v text=%q err=%v", changed, edit, doc.Editor.String(), err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "x \u212A tail" || doc.Editor.CanUndo() {
		t.Fatal("replacement did not have exactly one undo step")
	}
	for _, tc := range []struct {
		value, query string
		selection    text.Range
		options      SearchOptions
	}{
		{"aaaaa", "aa", text.Range{Start: 1, End: 3}, SearchOptions{MatchCase: true}},
		{"cat_ cat", "cat", text.Range{Start: 0, End: 3}, SearchOptions{WholeWord: true}},
	} {
		doc := New("fixture")
		doc.Editor.Insert(tc.value)
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(tc.selection)
		changed, _, err := doc.ReplaceCurrentWithEdit(tc.query, "changed", tc.options)
		if err != nil || changed || doc.Editor.String() != tc.value || doc.Editor.Selection() != tc.selection || doc.Editor.CanUndo() {
			t.Fatalf("refusal for %q changed source or selection: changed=%v err=%v", tc.value, changed, err)
		}
	}
}

func TestReplaceSelectedLiteralDoesNotAllocateForOtherMatches(t *testing.T) {
	doc := New("fixture")
	doc.Editor.Insert(strings.Repeat("cat ", 50_000))
	doc.Editor.ClearHistory()
	allocations := testing.AllocsPerRun(3, func() {
		doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
		changed, _, err := doc.ReplaceCurrentWithEdit("cat", "dog", SearchOptions{})
		if err != nil || !changed || !doc.Editor.Undo() {
			t.Fatalf("replace and undo: changed=%v err=%v", changed, err)
		}
	})
	t.Logf("allocations per replace and undo: %.0f", allocations)
	if allocations > 256 {
		t.Fatalf("one literal replacement allocated %.0f times for 50,000 matches", allocations)
	}
}

func TestReplaceSelectedRegexUsesCapturesAndOriginalContext(t *testing.T) {
	doc := New("fixture")
	doc.Editor.Insert("prefix item-12 suffix")
	doc.Editor.SetSelection(text.Range{Start: 7, End: 14})
	changed, edit, err := doc.ReplaceCurrentWithEdit(`\b(item)-(\d+)\b`, `$2:$1`, SearchOptions{Regex: true})
	if err != nil || !changed || edit != (ReplacementEdit{Start: 7, End: 14, Text: "12:item"}) || doc.Editor.String() != "prefix 12:item suffix" {
		t.Fatalf("replacement: changed=%v edit=%+v text=%q err=%v", changed, edit, doc.Editor.String(), err)
	}
}

func TestRegexReplaceAllExpandsCapturesAsOneUndoUnit(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("item-12 item-34")
	count, err := doc.ReplaceAllWith(`(item)-(\d+)`, `${2}:$1`, SearchOptions{Regex: true})
	if err != nil || count != 2 || doc.Editor.String() != "12:item 34:item" {
		t.Fatalf("replace count=%d err=%v text=%q", count, err, doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "item-12 item-34" {
		t.Fatalf("undo = %q", doc.Editor.String())
	}
}

func TestInvalidRegexLeavesTextAndSelectionUntouched(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("one two")
	before := doc.Editor.Selection()
	if _, err := doc.FindWith("(", SearchOptions{Regex: true}, false); err == nil {
		t.Fatal("invalid search regex succeeded")
	}
	if _, err := doc.ReplaceAllWith("(", "x", SearchOptions{Regex: true}); err == nil {
		t.Fatal("invalid replace regex succeeded")
	}
	if doc.Editor.String() != "one two" || doc.Editor.Selection() != before {
		t.Fatal("invalid regex modified the document")
	}
}

func TestZeroWidthRegexDoesNotTrapFindOrReplace(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("abc")
	options := SearchOptions{Regex: true}
	if found, err := doc.FindWith("^", options, false); err != nil || found {
		t.Fatalf("zero-width find: found=%v err=%v", found, err)
	}
	if count, err := doc.ReplaceAllWith("^", "x", options); err != nil || count != 0 || doc.Editor.String() != "abc" {
		t.Fatalf("zero-width replace: count=%d err=%v text=%q", count, err, doc.Editor.String())
	}
}

func TestMatchesWithReturnsByteSpansWithoutChangingSelection(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("猫 cat 猫")
	before := doc.Editor.Selection()
	matches, err := doc.MatchesWith("猫", SearchOptions{})
	if err != nil || len(matches) != 2 || matches[0].Start != 0 || matches[0].End != 3 || matches[1].Start != 8 || matches[1].End != 11 {
		t.Fatalf("matches = %+v, err=%v", matches, err)
	}
	if doc.Editor.Selection() != before {
		t.Fatal("highlight query moved the selection")
	}
}

func TestMatchesWithLimitKeepsMatchSemantics(t *testing.T) {
	tests := []struct {
		name, value, query string
		options            SearchOptions
	}{
		{"unicode whole word", "猫 猫咪 猫", "猫", SearchOptions{WholeWord: true}},
		{"unicode case fold", "k K K", "k", SearchOptions{WholeWord: true}},
		{"regex multiline anchors", "a\na\na", `(?m)^a`, SearchOptions{Regex: true}},
		{"regex zero width", "cat dog", `\b|cat`, SearchOptions{Regex: true}},
		{"regex captures", "a1 a2 a3", `a(\d)`, SearchOptions{Regex: true}},
		{"regex whole word", "cat_cat cat cats cat", `cat`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex whole word capture", "cat_dog cat dog DOG", `(cat|dog)`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex whole word anchored", "cat\ncat\ncat", `(?m)^cat`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex whole word boundary", "cat_cat cat", `\bcat`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex absolute start", "cat cat dog", `\Acat|dog`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex absolute end", "cat dog cat", `cat\z|dog`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex non-boundary", "猫cat xcat cat", `\Bcat|cat\z`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex empty moves by rune", "猫a猫aa", `a?|\z`, SearchOptions{Regex: true}},
		{"regex empty alternative priority", "cat 猫cat cat", `\b|cat`, SearchOptions{Regex: true}},
		{"regex with trailing quoted text", "cat cat ", `\b\Qcat`, SearchOptions{Regex: true, WholeWord: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc := New("new 1")
			doc.Editor.Insert(test.value)
			all, err := doc.MatchesWith(test.query, test.options)
			if err != nil {
				t.Fatal(err)
			}
			for limit := 0; limit <= len(all)+1; limit++ {
				spans, truncated, err := doc.MatchesWithLimit(test.query, test.options, limit)
				if err != nil || truncated != (len(all) > limit) || !reflect.DeepEqual(spans, all[:min(limit, len(all))]) {
					t.Fatalf("limit %d: spans=%+v truncated=%v err=%v; all=%+v", limit, spans, truncated, err, all)
				}
			}
		})
	}
}

func FuzzLimitedRegexAgreesWithFullSearch(f *testing.F) {
	f.Add(`(?m)^cat\b`, "cat_\ncat\ncat", uint8(2), true)
	f.Add(`\b|cat`, "cat \u732bcat cat", uint8(1), false)
	f.Add(`\Acat|cat\z`, "cat cat cat", uint8(3), true)
	f.Add(`a?|\z`, "\u732ba\u732baa", uint8(1), false)
	f.Fuzz(func(t *testing.T, pattern, value string, capHint uint8, wholeWord bool) {
		if len(pattern) > 128 || len(value) > 256 {
			return
		}
		options := SearchOptions{Regex: true, WholeWord: wholeWord}
		expression, err := compileSearch(pattern, options)
		if err != nil || expression == nil {
			return
		}
		limit := int(capHint % 8)
		all := searchSpans(value, expression, options)
		spans, truncated, err := MatchesInTextWithLimit(t.Context(), value, pattern, options, limit)
		if err != nil || truncated != (len(all) > limit) || !reflect.DeepEqual(spans, all[:min(limit, len(all))]) {
			t.Fatalf("limit %d: spans=%+v truncated=%v err=%v; all=%+v", limit, spans, truncated, err, all)
		}
	})
}

func TestRegexWidthAndSuffixSafetyProofs(t *testing.T) {
	tests := []struct {
		pattern    string
		consumes   bool
		suffixSafe bool
	}{
		{`(?i)a+?`, true, true},
		{`(?m)^a`, true, false},
		{`(?:a|b)+`, true, true},
		{`\bcat`, true, false},
		{`a|\b`, false, false},
		{`a?`, false, false},
		{`(?m)^`, false, false},
		{`cat$`, true, false},
	}
	for _, test := range tests {
		expression := regexp.MustCompile(test.pattern)
		if got := regexAlwaysConsumes(expression); got != test.consumes {
			t.Errorf("width proof for %q = %v, want %v", test.pattern, got, test.consumes)
		}
		if got := regexCanResumeOnSuffix(expression); got != test.suffixSafe {
			t.Errorf("suffix proof for %q = %v, want %v", test.pattern, got, test.suffixSafe)
		}
	}
}

func TestRegexWholeWordBatchBoundariesKeepAcceptedOrder(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert(strings.Repeat("cat_", 255) + " cat " + strings.Repeat("cat_", 256) + " cat " + strings.Repeat("cat_", 257) + " cat")
	options := SearchOptions{Regex: true, WholeWord: true}
	all, err := doc.MatchesWith("cat", options)
	if err != nil || len(all) != 3 {
		t.Fatalf("reference matches=%d err=%v", len(all), err)
	}
	for limit := 0; limit <= len(all); limit++ {
		spans, truncated, err := doc.MatchesWithLimit("cat", options, limit)
		if err != nil || truncated != (limit < len(all)) || !reflect.DeepEqual(spans, all[:limit]) {
			t.Fatalf("limit %d: spans=%+v truncated=%v err=%v, want %+v", limit, spans, truncated, err, all[:limit])
		}
	}
}
