# Scintilla Indicator Oracle

`testdata/oracle.cxx` runs the original `DecorationList` and `RunStyles`
implementations from Notepad++ v8.9.8.1, commit
`dd40fe45598791e9aa38f3aeb200e10fb3d6338a`. It generates 1080 primitive
replacement cases from six initial indicator configurations. No model of
Scintilla's range behavior is duplicated inside the oracle.

From the VivaPad repository, with that original checkout at `$NPP_SOURCE`:

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG \
  -I "$NPP_SOURCE/scintilla/include" -I "$NPP_SOURCE/scintilla/src" \
  tools/nppindicators/testdata/oracle.cxx \
  "$NPP_SOURCE/scintilla/src/Decoration.cxx" \
  "$NPP_SOURCE/scintilla/src/RunStyles.cxx" \
  -o /tmp/vivapad-indicator-oracle
/tmp/vivapad-indicator-oracle internal/indicators/testdata/scintilla-edits.tsv
go test ./internal/indicators -count=1
rm /tmp/vivapad-indicator-oracle
```

Relevant original Git blob IDs:

- `Decoration.cxx`: `53fbe486ed9a21b8cb86319bac4f8838fe38bd5a`
- `RunStyles.cxx`: `e4932c06627c344681cc956d909203be66997556`

The checked-in TSV is generated evidence. The reference source and temporary
compiler output are not bundled with the application.

Fixture SHA-256: `2e073a8b5ce46b17f909bf571ff11b3bdecfdf47bbba38972259beea8e9b4b92`.
On the validation Mac, Command Line Tools lacked its standard C++ headers;
the existing Homebrew LLVM compiler, headers and libc++ were used instead.
No system toolchain files were changed.

## Target Selection

`testdata/selection_oracle.cxx` runs original
`SelectionRange::MoveForInsertDelete` for 1260 empty/forward/backward selection
cases, applying deletion followed by insertion without virtual space.

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG -I "$NPP_SOURCE/scintilla/src" \
  tools/nppindicators/testdata/selection_oracle.cxx \
  "$NPP_SOURCE/scintilla/src/Selection.cxx" -o /tmp/vivapad-selection-oracle
/tmp/vivapad-selection-oracle internal/document/testdata/scintilla-selection.tsv
go test ./internal/document -run TestTargetSelection -count=1
rm /tmp/vivapad-selection-oracle
```

Original `Selection.cxx` blob: `b04820c6a18b8bd28c7e45a8a5f8d63590b94dc2`.
Selection fixture SHA-256:
`13a40b3b1a27e260c27876e8976d2c476401d3cc60f6b03ce3818a0d8ef27cb7`.

## Target Sequences

`testdata/target_oracle.cxx` composes original Decoration operations in the
forward/Undo/Redo order of adjacent Replace All targets and ordinary Column
Editor line-content targets. Its 15 rows pin gap-merging and EOL-run behavior;
Undo is not treated as a saved indicator snapshot.

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG \
  -I "$NPP_SOURCE/scintilla/include" -I "$NPP_SOURCE/scintilla/src" \
  tools/nppindicators/testdata/target_oracle.cxx \
  "$NPP_SOURCE/scintilla/src/Decoration.cxx" \
  "$NPP_SOURCE/scintilla/src/RunStyles.cxx" -o /tmp/vivapad-target-oracle
/tmp/vivapad-target-oracle internal/indicators/testdata/scintilla-target-sequences.tsv
go test ./internal/indicators -count=1
rm /tmp/vivapad-target-oracle
```

Target sequence fixture SHA-256:
`2c6e3bb9cb87cc3402f2fa08034107347c26966675b89fce213fc423fa68a94d`.

## Comment Sequences

`testdata/comment_oracle.cxx` uses the same original Decoration/RunStyles
implementations. Primitive boundaries are transcribed from pinned
`Notepad_plus.cpp::doBlockComment`, `doStreamComment` and `undoStreamComment`;
this does not execute the Windows application or replace native acceptance.
The eight scenarios cover line insertion/mixed toggle, stream-only line
insertion, selected/empty/blank block insertion and multiple/empty-pair block
removal. For every single initial interval and separated one-byte interval
pair, it records final forward, Undo and Redo results: 2181 rows in total.

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG \
  -I "$NPP_SOURCE/scintilla/include" -I "$NPP_SOURCE/scintilla/src" \
  tools/nppindicators/testdata/comment_oracle.cxx \
  "$NPP_SOURCE/scintilla/src/Decoration.cxx" \
  "$NPP_SOURCE/scintilla/src/RunStyles.cxx" -o /tmp/vivapad-comment-oracle
/tmp/vivapad-comment-oracle internal/document/testdata/scintilla-comments.tsv
GOWORK=off go test ./internal/document -run TestCommentTargets -count=1
rm /tmp/vivapad-comment-oracle
```

The application batches sorted delimiter targets in forward order. Original
stream uncomment removes each closing delimiter before its opening delimiter;
an empty body may also share the optional space. The fixture compares the
final indicator outcomes of those original operations with the app's actual
batch journal, including inverse operations. It does not claim identical
intermediate notifications, nested-pair discovery, Undo caret or save points.
An empty block insertion combines its adjacent strings into one insertion;
the original two-insertion outcome is covered separately.

Comment fixture SHA-256:
`a7318f148af495e1da637ade9ba39ae8fd49e47d87a36a7e81fcb4c9639cb043`.
Two independent generations produced identical bytes.

## Undo Save Points

`testdata/savepoint_oracle.cxx` executes the pinned original `UndoHistory.cxx`
for 73 committed-edit states: typing and deletion with zero net change, Save
coalescing barriers, branching, equal replacements, discarded Redo, explicit
history clearing, Unicode, grouped zero-net work, equal target batches and
empty edits. It replays the original returned actions against a byte string;
it does not reimplement history grouping or saved-position decisions.

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG \
  -I "$NPP_SOURCE/scintilla/include" -I "$NPP_SOURCE/scintilla/src" \
  tools/nppindicators/testdata/savepoint_oracle.cxx \
  "$NPP_SOURCE/scintilla/src/UndoHistory.cxx" -o /tmp/vivapad-savepoint-oracle
/tmp/vivapad-savepoint-oracle internal/document/testdata/scintilla-savepoints.tsv
GOWORK=off go test ./internal/document ./internal/searchfiles ./cmd/vivapad \
  -run 'Test.*SavePoint|Test.*SavedUndoBoundary|TestEqualReplacement' -count=1
rm /tmp/vivapad-savepoint-oracle
```

Fixture SHA-256:
`813d429a30b7efb17d01c4f9f8c5a24516d069131ef6cddb922081c96340468a`.
Viva's optional save-point tests use the identical fixture. Explicit engine
history clearing rebases the point; Notepad++'s loaded-dirty/save-point
notifications, native caret/selection history, tentative IME and EOL-conversion
Undo are separate acceptance gates. Documents retain their saved bytes and
encoding/EOL/recovery guards in addition to the engine point.

## Blank Operations

`testdata/blank_oracle.cxx` compiles original Decoration/RunStyles and applies
source-transcribed targets from `Notepad_plus.cpp::{doTrim,wsTabConvert,eol2ws}`
and Scintilla `Editor::LinesJoin`. Its 3579 forward/Undo/Redo states cover all
eight commands, unchanged lines, EOL ranges, whole changed-line TAB targets,
same-style deletion-gap merging and combined phases. The extra marked-word
case pins that Trim/Join can merge two runs and that Undo is not a style snapshot.

The same harness transcribes the pinned C++ whitespace conversion loop and
per-line caret calculation, generating 8100 UTF-8/ASCII states for widths
1, 2, 4 and 8, leading/all modes, existing TABs and every valid byte caret.
These are independently executed C++ source-derived fixtures, not a Windows
Notepad++ run or a claim that the entire original command handler was compiled.
Embedded NUL, legacy-byte and native caret-history behavior remain separate.

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG \
  -I "$NPP_SOURCE/scintilla/include" -I "$NPP_SOURCE/scintilla/src" \
  tools/nppindicators/testdata/blank_oracle.cxx \
  "$NPP_SOURCE/scintilla/src/Decoration.cxx" \
  "$NPP_SOURCE/scintilla/src/RunStyles.cxx" -o /tmp/vivapad-blank-oracle
/tmp/vivapad-blank-oracle \
  internal/document/testdata/scintilla-blank-targets.tsv \
  internal/document/testdata/scintilla-blank-tabs.tsv
GOWORK=off go test ./internal/document -run TestBlank -count=1
rm /tmp/vivapad-blank-oracle
```

Both fixtures were generated twice with identical bytes. SHA-256:

- Targets: `6b9f32000a4018d7a9dd95b397a2815e077a02582808ad500f3ccdce034b326a`.
- TAB/caret: `2ca7b8eb5acab8d2585fcc50ee888702ea7849b4db6a940a8d7b9a3c963e7109`.

## Source Line Operations

`testdata/line_oracle.cxx` includes original `Sorters.h`, Decoration and
RunStyles. A `codecvt_utf8_utf16<wchar_t>` bridge explicitly checks two code
units for an astral character, so the original sort classes compare Windows
UTF-16 units even on a host with 32-bit `wchar_t`. Win-only CRT/decimal/locale
declarations are not executed. Join/Split, deletion, blank-line insertion and
Locale-equal primitive boundaries are transcribed from the pinned handlers;
the entire original Windows application is not compiled by this harness.

The 5214 forward/Undo/Redo states pin insertion-only Split, EOL-only Join,
source terminal cleanup, exact/ASCII blank selection, final-copy consecutive
deduplication, intentional whole-block sort/any-dedup replacement, selected vs
whole-document EOF-empty participation, UTF-16 keys and Locale-equal history.

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG -Wno-deprecated-declarations \
  -I "$NPP_SOURCE/scintilla/include" -I "$NPP_SOURCE/scintilla/src" \
  -I "$NPP_SOURCE/PowerEditor/src/MISC/Common" \
  tools/nppindicators/testdata/line_oracle.cxx \
  "$NPP_SOURCE/scintilla/src/Decoration.cxx" \
  "$NPP_SOURCE/scintilla/src/RunStyles.cxx" -o /tmp/vivapad-line-oracle
/tmp/vivapad-line-oracle internal/document/testdata/scintilla-line-targets.tsv
GOWORK=off go test ./internal/document -run TestLineTargets -count=1
rm /tmp/vivapad-line-oracle
```

`testdata/line_marker_oracle.cxx` reuses those scenarios and executes original
CellBuffer/LineMarkers with every initial marker subset: 420 forward profiles.
The only platform shim prints diagnostics. Whole-buffer deletion clears markers;
partial deletion and insertion at a line start follow actual per-line merges,
not line-text matching. This fixture intentionally does not certify bookmark
Undo: the application's existing snapshot transitions are a known separate
source-alignment gap. It must not be confused with the indicator Undo oracle.

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG -Wno-deprecated-declarations \
  -I "$NPP_SOURCE/scintilla/include" -I "$NPP_SOURCE/scintilla/src" \
  -I "$NPP_SOURCE/PowerEditor/src/MISC/Common" \
  tools/nppindicators/testdata/line_marker_oracle.cxx \
  "$NPP_SOURCE/scintilla/src/CellBuffer.cxx" \
  "$NPP_SOURCE/scintilla/src/UndoHistory.cxx" \
  "$NPP_SOURCE/scintilla/src/PerLine.cxx" \
  "$NPP_SOURCE/scintilla/src/ChangeHistory.cxx" \
  "$NPP_SOURCE/scintilla/src/RunStyles.cxx" \
  "$NPP_SOURCE/scintilla/src/Decoration.cxx" \
  "$NPP_SOURCE/scintilla/src/UniConversion.cxx" -o /tmp/vivapad-line-markers
/tmp/vivapad-line-markers internal/document/testdata/scintilla-line-markers-forward.tsv
GOWORK=off go test ./cmd/vivapad \
  -run TestLineMarkerForwardProjectionMatchesOriginalCellBuffer -count=1
rm /tmp/vivapad-line-markers
```

Two generations of each file match byte-for-byte. SHA-256:

- Line targets: `b9cd1c9b17f73a68ae6e6504460f70babf7f5365546882c5bb51bdceb1820b39`.
- Forward markers: `9744b53bc759b3eb0e42ad82d69820133d41a5c3b050ab6cdd1d55f551ad412e`.

## Marker History

`testdata/marker_history_oracle.cxx` executes original CellBuffer/LineMarkers
and its actual UndoHistory actions, rather than reversing saved marker sets.
It extends the line-command scenarios with full deletion/equal replacement,
empty-buffer insertion, Unicode and two transient sequences with identical
initial/final bytes and primitive byte counts but different line-start facts.
Every initial marker subset runs both unchanged and with all live marks toggled
between the edit and Undo. The fixture contains 2652 states: 884 forward,
884 Undo and 884 Redo. Each output phase also checks that the replayed bytes
equal the original CellBuffer's bytes.

`internal/linemarks.Map` matches every row using ordered byte replacements and
the actual insertion-line-start/inserted-LF flags. It rejects an incomplete
sequence instead of applying only its retained suffix. Existing line-command
forward projection uses the same mapper. **The application's snapshot-based
bookmark Undo is not replaced yet:** complete editor batch/transaction history
metadata is still needed before this oracle establishes application acceptance.
The fixture's scope is normalized LF, not native CRLF/code-page notifications.

```sh
SOURCE_EXPORT=$(mktemp -d)
git -C "$NPP_SOURCE" archive dd40fe45598791e9aa38f3aeb200e10fb3d6338a \
  scintilla/include scintilla/src PowerEditor/src/MISC/Common/Sorters.h | \
  tar -x -C "$SOURCE_EXPORT"
"${CXX:-clang++}" -std=c++17 -DNDEBUG -Wno-deprecated-declarations \
  -I "$SOURCE_EXPORT/scintilla/include" -I "$SOURCE_EXPORT/scintilla/src" \
  -I "$SOURCE_EXPORT/PowerEditor/src/MISC/Common" \
  tools/nppindicators/testdata/marker_history_oracle.cxx \
  "$SOURCE_EXPORT/scintilla/src/CellBuffer.cxx" \
  "$SOURCE_EXPORT/scintilla/src/UndoHistory.cxx" \
  "$SOURCE_EXPORT/scintilla/src/PerLine.cxx" \
  "$SOURCE_EXPORT/scintilla/src/ChangeHistory.cxx" \
  "$SOURCE_EXPORT/scintilla/src/RunStyles.cxx" \
  "$SOURCE_EXPORT/scintilla/src/Decoration.cxx" \
  "$SOURCE_EXPORT/scintilla/src/UniConversion.cxx" -o "$SOURCE_EXPORT/oracle"
"$SOURCE_EXPORT/oracle" internal/linemarks/testdata/scintilla-marker-history.tsv
GOWORK=off go test ./internal/linemarks -count=1
rm -r "$SOURCE_EXPORT"
```

Fixture SHA-256:
`e640b22fadcbfb1d8a958da0683781ecba70f366583c1827f8bc2495ad8af41e`.
Two independent generations matched byte-for-byte. On the validation Mac the
compiler was `/usr/local/opt/llvm/bin/clang++`, as in the earlier source oracles.

## Actual Viva Line Context

`../../internal/linemarks/viva_line_context_test.go` drives real Editor transactions and
Undo/Redo, then feeds LineEditsSince into the marker mapper. It loads the
original forward mutation payloads and compares all 2652 original marker
history states, including live toggles and transient text. Save-point mode
retains equal-byte transaction units, as the application requires.

The verified app-compatible fork `ac36aac` now exposes
TrackLineEdits/LineEditsSince. Both the 2652 legacy and 5460 rich line-fact
states, including the 2049-step transaction, run in the normal application
suite without a workspace override or overlay. From the application root:

```sh
GOWORK=off go test -race ./internal/linemarks \
  -run '^TestVivaLine(Context|Facts)Match(es)?OriginalMarkerHistory$' -count=1
```

The current published dependency passes these producer/mapper checks under
race. Documents enable capture at creation/open/reload; dedicated tests prove
that batch Undo retains facts even before the first bookmark. Most-recent
large-action delivery is implemented, but application snapshot retirement,
per-action consumption and explicit gap handling remain pending. Local
nonzero linked-size/timing costs are documented; upstream head `188b400`
passed its performance gate. These tests do not certify the current bookmark
consumer or native application/pixel acceptance.

## Rectangle Sort Keys

`testdata/rectangle_sort_oracle.cxx` reuses the source-line harness and compiles
the pinned original Sorters.h classes and SelectionRange movement. The handler's
line-local byte offsets plus virtual spaces are transcribed from NppCommands;
the original sorter then slices UTF-16 units, not display cells. The 7784 cases
cover both endpoint directions, both sort directions, column ties, zero-width
suffix keys, short/empty lines, TAB/Unicode/NUL, decimal failure and terminal
empty-line participation. Single-row cases follow the original selection-count
branch, including whole-document fallback and the one-line numeric no-op.
A changed block also checks collapsed primary and
rectangular endpoints through actual original delete/insert movement.

The numeric and case Windows CRT calls use explicit host C-locale shims; they
are not Windows parser/case acceptance. Random's platform-specific engine and
Locale's separate implementation are not certified by this fixture. Indicator
and bookmark policy is exercised by existing full-block fixtures and Driver
tests, not new source indicator snapshots.

After exporting the same original source directories as the marker-history
example, compile with the original Selection.cxx, Decoration.cxx and
RunStyles.cxx, then generate and test:

```sh
"${CXX:-clang++}" -std=c++17 -DNDEBUG -Wno-deprecated-declarations \
  -I "$SOURCE_EXPORT/scintilla/include" -I "$SOURCE_EXPORT/scintilla/src" \
  -I "$SOURCE_EXPORT/PowerEditor/src/MISC/Common" \
  tools/nppindicators/testdata/rectangle_sort_oracle.cxx \
  "$SOURCE_EXPORT/scintilla/src/Selection.cxx" \
  "$SOURCE_EXPORT/scintilla/src/Decoration.cxx" \
  "$SOURCE_EXPORT/scintilla/src/RunStyles.cxx" -o "$SOURCE_EXPORT/rectangle-sort"
"$SOURCE_EXPORT/rectangle-sort" internal/document/testdata/scintilla-rectangle-sort.tsv
GOWORK=off go test ./internal/document -run '^TestRectangleSort' -count=1
```

Two independent generations match SHA-256
`f9e9af211b444c32ad00b0af52508792e9e3bcf01f17546dca31b9848fc4e999`.

## Locale Rectangle Handler

`testdata/locale_sort_oracle.cxx` compiles the actual original
`PowerEditor/src/MISC/Common/SortLocale.cpp`, blob
`c3d9a3329d89daa3ed71ecbc14e2533d454cad89`. Its quoted header is supplied by
`testdata/locale_adapter/SortLocale.h`, an explicit bounded Scintilla interface
adapter. The original method performs key extraction, stable row ordering,
EOF reconstruction, the real replacement decision and restoration calls.
Those branches are not rewritten as another sorting oracle.

The adapter supplies fixed-cell row endpoints for ASCII/TAB and the specified
CJK/emoji characters, and an ASCII case-insensitive numeric key stand-in.
Therefore the fixture certifies handler outcomes under that interface, not
LCMapStringEx, native font/midpoint/virtual-space pixels, arbitrary locale
weights, failure codes, legacy code pages or original Windows Undo caret
history. Application refusal and one-step Undo/Redo have separate Go/Driver
tests. The production collator remains the existing x/text implementation.

```sh
SOURCE_EXPORT=$(mktemp -d)
git -C "$NPP_SOURCE" archive dd40fe45598791e9aa38f3aeb200e10fb3d6338a \
  PowerEditor/src/MISC/Common/SortLocale.cpp scintilla/include | tar -x -C "$SOURCE_EXPORT"
"${CXX:-clang++}" -std=c++17 -DNDEBUG -Wno-deprecated-declarations \
  -I tools/nppindicators/testdata/locale_adapter -I "$SOURCE_EXPORT/scintilla/include" \
  tools/nppindicators/testdata/locale_sort_oracle.cxx \
  "$SOURCE_EXPORT/PowerEditor/src/MISC/Common/SortLocale.cpp" -o "$SOURCE_EXPORT/locale-sort"
"$SOURCE_EXPORT/locale-sort" internal/document/testdata/scintilla-locale-rectangle.tsv
GOWORK=off go test ./internal/document ./cmd/vivapad \
  -run 'TestRectangleLocale|TestLocaleRectangle' -count=1
rm -r "$SOURCE_EXPORT"
```

All 840 cases pass, including selected versus suffix keys, both vertical and
horizontal directions, widths 2/4/8, short and virtual rows, equal results,
covered terminal empty lines and single-row no-op behavior. Two generations
match SHA-256
`b5a4d59142de8641484136fac90bfc462c0ef8b60128fb63ad59680c63ea17b7`.

## Native Windows NLS Gate

Step 125 supplies a production Windows `LCMapStringEx` backend and a separate
portable injected-call contract test. The 840-case original-handler fixture
selects its Unicode key provider explicitly on every platform: its host key
stand-in must not become a golden test of real Windows collation weights.

On an actual Windows host, from the application repository in PowerShell:

```powershell
$env:GOWORK = "off"
go test ./internal/document -run 'TestNativeWindowsLocale|TestNLSKeys|TestLaterLocaleKeyFailure' -count=1 -v
go test ./internal/document ./cmd/vivapad -count=1
```

The native tests exercise neutral-language resolution, null user-default
locale names, digit/case/cultural ordering, embedded NUL and astral input,
GC around actual calls, and document/rectangle history. No literal sort-key
bytes are frozen: NLS data can vary by OS version and locale. They must run,
and the original Notepad++ same-host comparison must be recorded, before
claiming native acceptance. Compiling these tests into PE amd64/arm64 binaries
does not execute them.

API contract references:

- [LCMapStringEx](https://learn.microsoft.com/en-us/windows/win32/api/winnls/nf-winnls-lcmapstringex): explicit UTF-16 source lengths, byte destination/return counts, opaque keys including zero bytes and the two-call sizing protocol.
- [ResolveLocaleName](https://learn.microsoft.com/en-us/windows/win32/api/winnls/nf-winnls-resolvelocalename): neutral tags resolve to a specific Windows locale; output length includes the name terminator.
- [NLS header constants](https://github.com/mingw-w64/mingw-w64/blob/master/mingw-w64-headers/include/winnls.h): source flags combine to `0x08000418`, and user-default locale is null. `LOCALE_NAME_MAX_LENGTH` is 85 UTF-16 units in winnt.h.

The local injected contracts pass; the latest environment audit found no open
Motty sessions, saved-host inventory returned HTTP 404, and Wine/PowerShell
were absent. The native execution requirement remains pending.

## Marker Source-Line Facts Re-audit

`marker_history_oracle.cxx --line-facts` preserves the old fixture mode and
adds source line number, removed/inserted LF counts and original forward
mutation payloads. It executes the same original CellBuffer/LineMarkers and
UndoHistory, plus seven targeted and 64 deterministic seven-step scripts.
The current fixture also includes a 2049-step atomic insertion transaction;
all 5460 states are covered with every initial mark subset and live toggles.

The old byte-anchor mapper demonstrably fails a new case: remove LF from
`aa\nbb`, then insert LF at byte one. Original output retains line zero, not
line one. Therefore passing the earlier 2652 cases is not sufficient to claim
general marker history. The old fixture remains byte-identical.

Using the original source export and compiler command from Marker History,
generate the rich mode with:

```sh
"$SOURCE_EXPORT/oracle" internal/linemarks/testdata/scintilla-marker-line-facts.tsv --line-facts
GOWORK=off go test -race ./internal/linemarks -count=1
```

Rich fixture SHA-256:
`a43840f87ee68b3a522c0efc0fc9daf65301e9b4239a965b39db6b45b9e8db16`.
Two generations match. The actual-producer overlay now also includes
`TestVivaLineFactsMatchOriginalMarkerHistory`; it requires the revised prototype
with `Line`, `RemovedLines` and `InsertedLines`, not the previous flag-only
app-compatible experiment. Select that source explicitly in the temporary
workspace and run `-run '^TestVivaLine'`. Both probes pass under race.

No current application pin or snapshot history is retired by this proof.
Most-recent atomic delivery above the ordinary ring now passes forward/inverse,
exact-cursor and lifetime tests; the original large case exercises that path.
General gaps still fail closed. Fresh local costs are documented, but clean
publication, CI/review and actual app history migration remain gates; see
`marker-context-review.md` in the application spec package.

## Change-History Lines And Navigation

`build_change_navigation_oracle.sh` verifies raw Git blob hashes before
extracting and compiling actual `Document::GetMark`,
`Notepad_plus::changedHistoryGoTo` and the `Editor::MarkerPrevious` message
case. The adapter supplies normalized-LF line boundaries, original metadata
query rows, an unrelated bookmark bit and synthetic four-edition markers.
It does not instantiate a native Scintilla Document or exercise fold reveal,
font positions, native caret/selection or the no-match audible signal.

Export raw pinned source, retaining original CRLF bytes:

```sh
SOURCE_EXPORT=$(mktemp -d)
git -C "$NPP_SOURCE" archive dd40fe45598791e9aa38f3aeb200e10fb3d6338a \
  scintilla/include scintilla/src/Document.cxx scintilla/src/Editor.cxx \
  PowerEditor/src/Notepad_plus.cpp | tar -x -C "$SOURCE_EXPORT"
env CXX=/usr/local/opt/llvm/bin/clang++ sh tools/nppindicators/build_change_navigation_oracle.sh \
  "$SOURCE_EXPORT" "$SOURCE_EXPORT/build"
"$SOURCE_EXPORT/build/change-navigation-oracle" lines \
  internal/changehistory/testdata/scintilla-change-history.tsv "$SOURCE_EXPORT/original-lines.tsv"
"$SOURCE_EXPORT/build/change-navigation-oracle" lines \
  internal/changehistory/testdata/scintilla-change-history-span-fixed.tsv "$SOURCE_EXPORT/experiment-lines.tsv"
"$SOURCE_EXPORT/build/change-navigation-oracle" navigation "$SOURCE_EXPORT/navigation.tsv"
cmp "$SOURCE_EXPORT/original-lines.tsv" internal/changehistory/testdata/scintilla-change-lines.tsv
cmp "$SOURCE_EXPORT/experiment-lines.tsv" internal/changehistory/testdata/scintilla-change-lines-span-fixed.tsv
cmp "$SOURCE_EXPORT/navigation.tsv" internal/changehistory/testdata/npp-change-navigation.tsv
GOWORK=off go test -race ./internal/changehistory -count=1
rm -r "$SOURCE_EXPORT"
```

Two independent generations of each fixture match byte-for-byte:

| Fixture | Scope | SHA-256 |
| --- | --- | --- |
| `scintilla-change-lines.tsv` | 2293 unchanged-original states / 9170 line masks | `a9b9f800b734b3dd4b4e9aa3daee271c5df2c451675bb17eb55e4843210119b3` |
| `scintilla-change-lines-span-fixed.tsv` | 8299 separately corrected-reference states / 32912 line masks | `c376acb8faeaf92e423698e215070415d57c0def41d8bc5e60ee7a9d9004d378` |
| `npp-change-navigation.tsv` | All marker patterns, cursors and both directions for 1–9 lines: 16388 cases | `1626d19024dc686dadb4f91cb7a0c63a9c7d299336d4e450aca823b7007326e7` |

The experiment label is copied from its input and remains separate from the
unchanged-original prefix. Application history delivery and UI commands are
still missing. See `change-history-source-review.md` and
`change-history-editor-contract.md` in the active spec package.
