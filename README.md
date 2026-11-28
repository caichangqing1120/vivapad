# VivaPad

VivaPad is a cross-platform desktop text editor written in Go with
[Viva](https://github.com/vivaui/viva). Its reference is the classic light
Notepad++ layout. This is a runnable first slice, **not** a one-to-one
Notepad++ implementation or a pixel-matched reproduction yet.

## Development

Requirements start in GitHub Issues. Comment `/claim` and wait for the bot's
assignment confirmation before implementing; link the Issue in a focused PR
after verification. See [CONTRIBUTING.md](CONTRIBUTING.md) for CI, bot commands
and review rules. Detailed Specs, development plans, verification records and
screenshots are kept locally and are not published in this repository.

## Run

Requires Go 1.26 or newer and the platform prerequisites listed by Viva.

```sh
GOWORK=off go run ./cmd/vivapad
GOWORK=off go run ./cmd/vivapad path/to/file.txt
GOWORK=off go test ./... -count=1
```

On macOS, the latest locally packaged app is at `build/VivaPad.app`
and can be opened with `open build/VivaPad.app`. Only this current package is
retained after validation; older milestone builds are removed. It is unsigned;
distribution to other Macs needs
signing and notarization. Build for Apple Silicon with
`GOWORK=off go tool viva package --target darwin --arch arm64 --format app --no-sign -o build/VivaPad.app`,
then run `go run ./tools/icon -app build/VivaPad.app` to preserve the approved
32/64/128/1024 PNG renditions inside the macOS icon container;
the package command otherwise defaults to amd64.

The macOS bundle, Linux desktop package, and Windows installer advertise
VivaPad in Open With for `.txt`, `.md`, `.markdown`, and `.json` files.
They do not set it as the default editor. Windows and Linux pass the selected
file to a new VivaPad process; macOS can also open it in a running window.

- **Settings > Style Configurator...** previews editor font size and per-theme colors before saving.
- **Language > User Defined Language...** defines custom keywords, delimiters, and file extensions. **Add Preset** offers eight built-in syntax packs. **Import JSON** accepts version-1 bundles and lets you skip, overwrite, or rename same-name languages. Renaming keeps existing extension assignments in place. **Export JSON** saves the selected language or all saved languages to a shareable bundle. Saved languages also appear in the Language menu.
- **View > Projects** groups file references. Removing a reference or group leaves the files on disk.
- **File > Open in New Window** opens an independent instance with the current saved file, or a blank document. When an open file changes on disk, the editor offers reload, comparison, or ignore; reloading a dirty tab still requires confirmation.

The source requires official Viva commit `95ea459` and temporarily replaces it
with published fork commit `ac36aac` for the pending clipboard, indentation
guide, soft-wrap marker, character-representation, colored mark-layer, primitive
edit-journal, ordered target-batch, nonmutating preflight, staged editor
transactions, optional Undo save points and source line/LF facts. The fork also contains native save checkbox and
always-on-top support, now merged upstream.
`GOWORK=off go test ./...` and packaging use that reproducible revision.
Viva PRs #139, #142, #143, #146, #147 and #151 have merged;
[PR #153](https://github.com/vivaui/viva/pull/153) covers the clipboard override;
[PR #160](https://github.com/vivaui/viva/pull/160) and
[PR #161](https://github.com/vivaui/viva/pull/161) have merged;
[PR #162](https://github.com/vivaui/viva/pull/162) for CodeEditor indent guides and
[PR #165](https://github.com/vivaui/viva/pull/165) for soft-wrap end markers have merged.
[PR #166](https://github.com/vivaui/viva/pull/166) provides display-only code representations.
[PR #168](https://github.com/vivaui/viva/pull/168) adds caller-colored CodeEditor mark layers;
its current draft head has 14/14 successful checks (2026-11-10).
[PR #169](https://github.com/vivaui/viva/pull/169) adds exact UTF-8 edit cursors, including
disjoint batch Undo/Redo primitives, optional source-ordered target batches with
atomic selection, isolated `Editor.Transact` commits for validated macro replay,
explicit saved Undo boundaries and optional source line/LF contexts. Current
draft head `188b400` has 14/14 successful checks, including performance and
Darwin, in CI `42663655048` and website `42663655044` (2026-11-10). Earlier
template, benchmark and disk-image failures are recorded in the verification
history, not current red checks. No PR has been merged by this task.

The application-compatible fork preserves the existing UI API. Documents
capture line history from creation/open/reload, before the first bookmark,
so old batch Undo has real line facts. Actual-producer checks are now in the
normal suite: 5460 rich original CellBuffer states plus 2652 legacy states.
The application now consumes the actual line facts for bookmark movement and
Undo/Redo rather than restoring saved marker sets. It passes the 5460 original
states through real application commands, including live changes and the large
action. Explicit journal gaps retain unmapped marks, show an error and refuse
bookmark mutation/session export until reset, rather than guessing or dropping
data. This is normalized-LF local evidence, not complete native history parity.
The optional line producer
has measured nonzero timing/linked-size cost, documented in the verification
report; its green upstream CI is not native or pixel acceptance.

The Change History metadata engine matches 2293 states from the unchanged
original before its reproducible deletion-stack failure. Line projection also
matches 9170 line masks from those states; pure Next/Previous navigation matches
16388 cases compiled from the actual original function, including block skipping
and asymmetric wrapping. A separately labeled
one-line correction experiment matches 8299 checked-in states and 129043
states in the expanded independent review. VivaPad uses this engine with
Viva's edit history. Missing history is reported rather than displayed as an
unchanged document. These comparisons cover the history engine and navigation
rules; they do not establish complete native behavior or pixel parity.

For repeatable, nonintrusive UI testing:

```sh
go build -tags vivahead -o /tmp/vivapad-head ./cmd/vivapad
VIVA_HEADLESS=1 /tmp/vivapad-head
# Then enter: click "Text editor", type hello, shot /tmp/vivapad.png, layout, quit
```

### Command Line Arguments

```sh
vivapad notes.txt "another file.txt"
vivapad --help
vivapad -- --help
```

Pass file paths to open them at startup. With no file arguments, VivaPad
reopens its previous session if available. A leading `-h` or `--help` prints
usage and exits without opening a window. Use a leading `--` to open a file
whose name would otherwise be interpreted as help, as in the last example.

### Large files and live logs

- Under 16 MiB, files open in the full editor. From 16 through 64 MiB, the
  lightweight editor keeps editing, Undo, and Save, but disables word wrap,
  syntax highlighting, Change History, and automatic crash recovery snapshots.
- Over 64 MiB, the read-only UTF-8 viewer displays a window of lines while
  building the line index in the background. Scroll, use **Go to Line** (which
  also accepts `first` and `last`), or select and copy text in the visible window.
  Long lines are clipped in the viewport, not in the search engine.
- **Find Next**, **Find Previous**, and **Find All** search one logical line at a
  time, with case, whole-word, and regular expression options. **Find All** lists
  up to 2,000 results; choosing a result reveals the match. Press Escape to
  cancel a search in progress.
- **View > Monitoring (tail -f)** follows newly appended text without blocking
  the process writing the log. Closing the tab stops monitoring and releases
  the file handle.

### Programmer tools

- **Tools > JSON / XML** formats, minifies, or sorts selected JSON (or the
  whole document if nothing is selected). XML formatting preserves mixed
  content and `xml:space` sections. Parse errors leave the source unchanged.
- **Tools > Encode / Convert** works on the selection or current line.
  Timestamp conversion also recognizes a Unix timestamp at the caret; dates
  use local time unless an offset is supplied, and produce seconds followed
  by milliseconds. URL conversion uses query/form escaping (`+` for spaces).
  Each successful edit keeps the document's line ending and undoes in one step.
- **Tools or View > Compare** compares two split editors, or the current buffer
  with a read-only disk snapshot. Vertical scrolling follows corresponding
  lines. Use **Next Difference** and **Previous Difference** with
  Ctrl+F8 / Ctrl+Shift+F8 (Cmd on macOS). **Clear Compare** restores the normal
  view without changing either file.

### Column selection

Column selection marks the same range of columns on several lines. Use any
of these methods:

- Hold Alt (Option on macOS) and drag across the lines.
- On Windows or Linux, hold Alt+Shift and use the arrow keys. On macOS,
  first choose **Edit > Begin/End Select in Column Mode** or press
  Option+Shift+B, then use Option+Shift+arrow keys.
- Place the caret at the first corner and choose **Edit > Begin/End Select
  in Column Mode**. Move the caret to the opposite corner and choose the
  same command again.

**Edit > Column Mode Help** opens this section.

## Current behavior

- Documents track an explicit saved Undo boundary. New edits that return to the
  same text, equal-target replacements and zero-net macro playback still mark
  the document modified; Undo must reach the saved boundary to clear it. Save
  preserves Undo/Redo and breaks typing coalescing, while Save Copy and failed
  writes leave the source point unchanged. Branching and history pruning cannot
  silently mark discarded saved data clean. This matches 73 states generated by
  original Scintilla UndoHistory; exact native caret, IME, EOL and reset
  notifications remain separate acceptance gaps.
- Search > Change History offers Go to Next Change, Go to Previous Change and
  Clear Change History. Clear removes change markers and Undo/Redo history
  while keeping unsaved edits. Under Settings, Change History can be shown in
  the margin (the default), as indicators, in both, or turned off.
- Search > Find characters in range... scans the active editor's UTF-8 bytes
  with ASCII, non-ASCII and validated custom ranges, direction and optional
  wrap. Hits select the containing Unicode character without changing text;
  exact legacy-code-page and Scintilla byte selection remain unverified.
- Edit > Column Editor... inserts literal text or formatted number sequences
  from the ordinary caret column to EOF or within a rectangular selection,
  padding short lines and keeping the change in one Undo unit. Decimal, hex,
  octal and binary modes are available. The ordinary path follows original
  per-line content targets, excluding EOL bytes, rather than rewriting the
  entire suffix or pretending they are point insertions. Selection movement
  matches 1260 original Scintilla cases; exact original Undo caret mode remains
  unverified. See [Column selection](#column-selection) for how to select a
  rectangle. Text, IME commits, single-line paste and deletion affect each
  row. Raw per-row column Copy/Cut and multiline paste work across rows;
  Tab-interior positions snap to the nearer boundary. Local Undo/Redo
  restores the matching rectangle state.
  Shared-clone edits keep selection ownership in each view. On Windows,
  rectangular Copy/Cut writes Scintilla's clipboard markers; a marked paste
  at an ordinary caret distributes rows. Cross-process Notepad++ and font-pixel
  parity remain open. Disk reload preserves clone rectangles when their selected
  rows are unchanged and the line count is stable; changed selected rows clear
  them, and the old undo history is always released.
- Settings > Preferences... opens Editing and View pages for supported tab,
  indent, wrapping, whitespace, line-number and zoom-sync settings. Changes
  apply immediately and persist privately; the dialog gates editing shortcuts
  and file drops. The rest of Notepad++ Preferences, Style Configurator and
  Shortcut Mapper remain incomplete.
- Run > Run... parses quoted executable/arguments, expands eleven active-file
  variables after tokenization and starts the process only on explicit Run or
  Enter. Saved commands persist in private storage and replay from a bounded,
  scrollable menu. Cancel and invalid commands never launch; the 640x400 dialog
  and menu have headless screenshots. On Windows, `%NAME%` variables expand
  within parsed arguments on launch or saved replay. An unknown variable
  prevents launch. Windows file associations, shortcuts and original pixel
  matching remain open.
- Tools > MD5/SHA-1/SHA-256/SHA-512 can hash UTF-8 text or each line, stream
  selected files and copy their digest report, or copy one selection's digest
  directly. The selection uses UTF-8 buffer text with its original line endings.
  Empty or multiple selections leave the clipboard untouched. Hashing does
  not modify or save editor buffers. Native ANSI behavior and exact Windows
  dialog and picker behavior are not yet accepted.

- Multiple tabs; new/open/save/save as/save all/close; save, discard, or cancel when
  closing a modified document. The native file picker and file drop can open
  files. Opening the same file selects its existing tab. Normal close remembers
  open file paths and the active tab; explicit file arguments take precedence
  on the next launch. Unsaved contents are backed up separately to a private
  local `recovery.json` and restored after abnormal termination. A disk conflict
  restores as an untitled copy instead of overwriting the changed file. The
  versioned backup is limited to 32 MiB, uses 0600 file permissions, and is
  cleared after a normal Save/Discard close.
- File > Close All and Close Other Tabs process open tabs in order. Dirty tabs
  still require individual Save, Discard or Cancel decisions; a canceled Save As
  stays on the same prompt. Cancel stops the remaining batch, without undoing
  tabs already confirmed closed. The active tab and split-pane ownership are
  retained for Close Other Tabs.
- File > Close Multiple Documents closes tabs to the left or right of the
  current tab, or only unchanged tabs, within the active view. It snapshots
  candidates from right to left, asks about each dirty tab, and detaches a
  clone instead of deleting the document still open in the other view. Each
  tab has a pin control; Close All but Pinned Documents visits both views,
  retaining pinned tabs and the existing dirty-tab confirmation gate.
- File > Save a Copy As writes the current buffer and encoding to a separate
  path atomically, while keeping the active tab's name, source path, dirty
  state and undo history. It refuses the source or another open tab's path,
  including aliases through symlinked directories.
- File > Rename moves an existing file after source/destination preflight,
  preserving unsaved edits, undo, bookmarks and pane ownership. Replacing an
  existing destination requires confirmation; an untitled tab instead changes
  only its validated tab name. Recent and automatic session paths follow a
  successful move. Private VivaPad data files cannot be rename targets.
  Cross-volume moves and OS-level overwrite races remain
  outside local verification.
- File > Move to Recycle Bin confirms the saved file name and location, warns
  before discarding dirty edits, and rechecks disk identity and bytes before
  asking the OS to move the file. Success closes every clone and removes its
  recent/closed history entries; cancel or failure keeps the tabs. macOS uses
  Foundation Trash, Windows 8+ requests the Recycle Bin through
  `IFileOperation`, and Linux uses `gio trash` when available. There is no
  permanent-delete fallback. Windows/Linux native
  behavior and original dialog pixels remain unverified.
- File > Print... uses the system dialog on macOS and Windows, initially
  choosing Selection for a nonempty selection. Print Now uses the whole live
  buffer and the default printer. Both backends paginate Unicode text against
  printer metrics with line numbers and Chroma colors; macOS submits through
  NSPrintOperation and Windows through GDI. A macOS process-scoped Save PDF
  verified an A4 page without adding an office job. The Windows backend is
  cross-compiled but has not been run with a virtual printer. Linux Print...
  uses the desktop print portal to choose paper, selection, ranges and copies,
  then submits an embedded-font PDF through a file descriptor; Print Now
  queries CUPS for the default printer's page and margins before submitting
  through `lp`. Both Linux paths are cross-compiled but lack physical runtime
  verification. Missing portal/CUPS services report an error without spooling.
  Settings > Preferences includes Print and Header/Footer pages for color
  modes, line numbers, form feeds, millimetre margins, six text positions and
  seven original variables. Print Fonts adds independent 6-14pt header/footer
  sizes and bold/italic. These persist privately and apply to the next print
  job. Font family selection, Linux PDF native font variants, exact Scintilla
  colors and original dialog pixels remain incomplete.
- File > Sessions exports and additively loads a source-compatible subset of
  Notepad++ XML sessions: saved file paths, main/sub views, active tabs,
  supported code pages and selections. Dirty tabs block export so their
  unsaved contents are not silently omitted; import preflights all files before
  changing the workspace and preserves existing dirty tabs. Bookmark lines
  plus user read-only and pinned flags round-trip in XML. When the workspace
  dock is visible, Save Session defaults to including its roots, selection and
  expanded directories in the original FileBrowser subtree; unchecking the
  option omits that subtree. Missing or escaping included folders fail
  preflight. Windows/macOS use a native save-picker checkbox, and Linux uses
  the file chooser portal when it supports choices; an unsupported picker
  falls back to VivaPad's option dialog. Backup buffers remain unsupported;
  supported fold headers are restored after validation. The private automatic JSON
  session remains separate.
- File > Open Folder as Workspace... and Folder of Current Document as Workspace
  add roots to a left dock. Expand folders, open files, add another root, refresh
  the listing, or hide/reopen the dock through View. Roots and visibility survive
  normal close in private session v6; missing roots are skipped on restart.
  Symlink directories are not followed. Expanded directories auto-refresh
  on create, remove and rename events through fsnotify; collapse or hide
  releases their native watches. Refresh remains available when watching
  fails. Recursive watching of collapsed branches and the original panel
  context menu are not supported.
- File > Open in Default Viewer passes a saved document path to Viva's system
  document opener without saving dirty edits; unsafe executable and missing
  paths are refused. Open Containing Folder > Explorer requests selection of
  an existing file in Finder/Explorer, or opens its parent if the file disappeared; Linux
  opens the parent directory without guaranteed selection. The same submenu
  adds the parent to Folder as Workspace. Windows cmd/PowerShell entries remain
  disabled pending a tested native console-window implementation.
- File > Reload from Disk refreshes a clean tab after an external change. A
  modified tab requires explicit confirmation before its edits are discarded;
  Cancel or a failed read leaves it untouched. Legacy files reload with their
  saved disk encoding, and split-view tab ownership and language remain intact.
- File > Recent Files keeps the ten latest successfully opened or saved paths
  in a private versioned history. Reopening an existing path moves it to the
  top; explicitly opened legacy files retain their code page. Session restore
  does not reorder the list, missing files leave the current tab untouched,
  and the submenu can clear its history. It also restores the most recently
  closed saved path (Control/Command+Shift+T), from a separate private history;
  discarded untitled content is not represented as recoverable.
- File > Exit uses the native window's Save/Discard/Cancel gate. A canceled
  Save As leaves the window open; confirmed close saves the private session
  and releases window-owned watchers.
- Native text and IME through Viva, selection, undo/redo, clipboard commands,
  Delete, keyboard shortcuts, auto-indent, bracket pairing, line editing, comment
  toggling, word wrap, horizontal scrolling, go-to-line, zoom and line numbers.
  Edit > Delete removes selected text or the next grapheme at the caret without
  touching the clipboard; an EOF no-op does not add a macro step.
  Edit > Begin/End Select (Primary+Shift+B) stores a view-local anchor and
  selects forward or backward on the next invocation; the column-mode variant
  remains unavailable without rectangular selection.
  Edit > Read-Only in Notepad++ protects the current or all open buffers from
  text edits while preserving selection, copy and Save of existing dirty text.
  The flag follows reload, XML sessions, private sessions and dirty recovery;
  it does not change the OS file's read-only attribute.
  The adjacent File Read-Only Attribute command changes the saved file's disk
  attribute separately. Existing read-only files block editor changes and
  overwrite Save, but a dirty buffer can still use Save As to another path.
  On macOS/Linux, this clears write bits and restores only owner-write on
  the next toggle; Windows ACL behavior still needs a physical test.
  Edit > Insert offers short/long/custom date-time insertion at the caret or
  over a selection. Settings > Date Time Format... persists the custom token
  pattern (default `yyyy-MM-dd HH:mm:ss`) and optional date-before-time order.
  Standard date words and all Windows custom token variants are not yet a
  native `GetDateFormatEx`/`GetTimeFormatEx` match on other platforms.
  Edit > Comment/Uncomment adds or removes one line-comment prefix on selected
  nonblank lines or the caret line, with a single undo step. Go/C-family,
  JavaScript, TypeScript, Java and Rust use `//`; Python, shell, YAML and TOML
  use `#`; SQL uses `--`. Block Comment/Uncomment wrap the selected text or
  current line and remove matched delimiters (`/* */` in C-family/CSS/SQL,
  `<!-- -->` in HTML); line-only languages fall back to their line prefix.
  Toggle Single Line Comment and Primary+/ now act independently on each
  selected line, including a mix of commented and uncommented lines. CSS/HTML
  use per-line block delimiters. Direct comment commands now edit only their
  delimiters, preserving untouched five-style/Find Mark ranges and the mapped
  selection through Undo/Redo; 2181 original Scintilla Decoration outcomes
  cover forward/Undo/Redo, empty bodies and multiple removed pairs. Composition
  and restricted editors refuse the entire action. New recordings retain the language delimiters
  for macro replay, while old Viva CodeStep macros keep their old behavior.
  Macro replay now preserves the actual staged edits instead of committing a
  whole-buffer replacement; individual command mappings remain separately audited.
  Nested/dialect-specific stream behavior, all lexer styles and Windows
  shortcut runtime remain unverified.
  Edit > Line Operations sorts selected logical lines (or the whole buffer when
  nothing is selected) in case-sensitive Unicode order, or removes duplicate
  lines while keeping their first occurrence. These changes are one undo unit
  and preserve the source EOF rules and document's saved LF/CRLF format. The same
  submenu also has stable case-insensitive ascending/descending sort, removal
  of empty versus whitespace-only lines, removal of consecutive duplicates,
  Join Lines, Reverse Line Order and Insert Blank Line Above/Below Current.
  It also offers ascending/descending integer-aware natural sort, fixed-locale
  decimal comma/dot sort and UTF-16 code-unit length sort. Unparseable numeric
  prefixes report the first failing line without changing the document. At
  compact sizes the long submenu has wheel and up/down navigation.
  Classic sorts now accept rectangular selections: they reorder only the
  covered whole lines using the source's byte-derived UTF-16 column keys,
  including short lines, virtual space, suffix keys and stable column ties.
  Reverse and Randomize use the same rectangle line scope. Numeric failure and
  editor refusal keep the source and rectangle unchanged; Undo/Redo retain
  owning-view rectangle state across a shared clone. A collapsed single-row
  rectangle follows the original whole-document fallback. All 7784 generated
  original-sorter/selection cases pass. Windows numeric/case/random behavior,
  exact Windows Locale keys and native selection-history pixels remain
  acceptance work.
  On macOS/Linux, Locale sorting uses Viva's preferred language and
  `x/text/collate` with case-insensitive numeric keys; that fallback is not a
  byte-for-byte Windows NLS match. Randomize Line Order uses a time-seeded
  shuffle, not the original C++ random engine.
  Locale's rectangular path now uses each row's actual selected key (or its
  suffix for an empty rectangle), includes a covered terminal empty line and
  restores the sorted boundary rows' endpoints and virtual spaces without
  collapsing the rectangle. A single row is a no-op; an eligible equal-byte
  result still changes real history and the saved boundary. Both menu
  directions and shared-clone Undo/Redo are tested. The original SortLocale.cpp
  handler matches 840 fixture cases through an explicit host key/geometry
  adapter; this does not certify Windows NLS or native virtual-space pixels.
  Windows builds now obtain Locale keys through `LCMapStringEx` using the
  original flags and the Windows user-default locale; explicit language tags
  pass through `ResolveLocaleName`. Empty inputs bypass NLS, UTF-16 counts
  include embedded NUL/surrogate pairs, and returned byte keys stay opaque.
  Any key-generation failure refuses the complete operation without clearing
  existing Redo or changing selection/history. Other platforms retain x/text
  through owned keys. Native Windows tests are included and cross-built for
  amd64/arm64, but have not run on Windows yet, so native NLS acceptance remains
  open. The materialized-key path trades extra temporary memory for less
  repeated comparison work; local measurements are recorded in verification.
  Split Lines inserts EOLs at the active Viva editor's painted soft-wrap
  boundaries without replacing the untouched body; Join changes only the source
  internal EOLs. Both normalize the original logical-line selection and keep its
  final state in the same Undo unit. Scintilla's edge-column and font metrics
  remain unmatched.
  Empty/blank deletion follows the exact selection and terminal-line cleanup;
  Blank means ASCII space/TAB, not NBSP. Consecutive EOF duplicates preserve the
  final unterminated copy's indicators. Classic whole-document sort/reverse and
  arbitrary deduplication include the terminal empty line; selected and Locale
  sorting use their distinct source EOF rules. UTF-16 lexicographic keys and
  source whole-target style loss are retained. Locale accepts equal bytes as a
  real replacement and restores its moved-line caret. Forward line bookmarks
  follow original CellBuffer effects, not matching text; complete bookmark Undo
  is source-compared locally, not claimed as complete original native parity.
  The application now consumes full source line/LF history: all 5460 original
  states pass through its forward/Undo/Redo path, with separate 2652-state
  producer checks. Macro draft marks are consumed before recorder teardown;
  no previous marker sets are restored. Move Lines uses the original ordered
  delete/insert and terminal-LF steps instead of a whole-block swap. Its exact
  original caret/selection behavior remains open.
  Edit > Blank Operations trims ASCII spaces/TABs, joins selected EOLs, and
  converts TABs/spaces at configured stops; each of its eight commands is one
  undo unit and preserves file encoding and LF/CRLF on save. Trim edits only
  whitespace, Join edits only EOLs, and TAB conversion replaces only changed
  line content, as in the original. Untouched styles follow actual source
  primitives rather than a whole-block replacement or saved indicator snapshot.
  Combined Trim/Join validates on a private draft and commits once. Active
  rectangles, composition and restricted editors refuse without partial edits.
  The source comparison covers 3579 Decoration and 8100 TAB/caret states;
  Windows runtime, embedded-NUL/code-page edges and exact native history remain
  separate acceptance gaps.
  Edit > Copy to Clipboard copies the current path/name/directory or all tab
  names/paths; all-list output is CRLF-delimited and deduplicates split clones.
  Edit > On Selection > Open File starts a new VivaPad process for a selected
  file or path at the caret; Open Containing Folder reveals an existing file or
  directory in the platform file manager. Both preflight the path and preserve
  the current dirty tabs and split views. Windows Explorer behavior and
  original pixels still need physical comparison.
  Edit > On Selection > Redact Selection masks each selected Unicode character
  with `█`, or `●` while Shift is held; legacy code pages use `#`/`.`. It
  preserves physical line endings and supports ordinary and rectangular
  selections as one undo unit. Arbitrary independent Scintilla selections and
  original Windows menu pixels remain unverified.
  Edit > Multi-select All/Next has four case/whole-word modes, word-first
  selection, wrap, Undo Latest and Skip Current. The default shortcuts are
  `Primary+Shift+D` for Next, `Primary+Shift+L` for All (both whole-word,
  case-insensitive), `Primary+Shift+J` for Skip, and `Escape` to keep only the
  primary caret. Undo Latest has no default binding. All five can be changed
  in Shortcut Mapper. `Primary` is Command on macOS and Ctrl on Windows and
  Linux; `Primary+D` still duplicates the line.
  Alt+Click adds or removes a secondary caret; Alt+Drag still selects a
  column. Left, Right, Home and End move every caret. Paste distributes lines
  in document order if their count matches the number of carets; otherwise it
  inserts the entire clipboard at each caret. Typing, deletion, paste and
  redaction are single undo steps that restore the selections and file line
  endings. IME composition safely returns to the main caret.
  Edit > Convert Case changes only selected text to UPPERCASE, lowercase,
  Proper Case force/blend, Sentence case force/blend, inverted or random case,
  including Unicode letters, as one undo unit. Primary+Shift+U and Primary+U
  provide the upper/lower shortcuts; no selection is a no-op. Exact Windows
  locale mappings and the source C++ random sequence are not verified.
  Edit > Auto-Completion > Word Completion (Primary+Enter) scans the current
  buffer. One match inserts directly; multiple matches use a caret popup with
  arrows, Enter/Tab, Escape, pointer selection and continued typing to filter.
  It supports Unicode words, one-step Undo and split views; exact original
  Scintilla word boundaries are not yet reproduced.
  Edit > Auto-Completion > Path Completion (Primary+Shift+Space) lists file and
  directory names for a path at the caret. Absolute, `~/`, quoted and
  saved-file-relative paths keep their typed form; accepting a directory adds
  the platform separator. The scan is capped at 2000 entries, and the popup
  shows distinguishable filenames while preserving the full path on selection.
  Original Windows drive-path runtime behavior remains unverified.
  Settings persists font size, wrap, line numbers, tab width, indentation mode,
  auto-indent and bracket pairing in a private versioned file. Language-default
  indentation keeps ordinary text tabs and code-language spaces unless overridden.
  View > Split View shows two editors with independent tabs and scrolling plus
  a draggable divider. Move/Clone to Other View and Ctrl+Tab work within the
  active view; a clone shares text and undo history, not scroll position. On
  resize the selected tab remains visible without overriding manual tab-strip
  navigation. A normal close remembers per-view tabs, active view and divider
  position.
- `Primary+Shift+P` opens the Command Palette; `Primary+P` searches open tabs,
  project files, and workspace folders. Search commands in English or Simplified
  Chinese, then use the arrow keys and Enter to choose a result. Both shortcuts
  are configurable in Shortcut Mapper; existing custom bindings are kept.
- Macro > Record/Stop/Playback records the latest single-document text input,
  IME commits, paste and supported code/line commands. `Primary+Shift+R`
  toggles recording and `Primary+Alt+P` plays back the last macro. Editor menu
  actions restore active-view input, so typing and Undo work without re-clicking
  the editor; dialogs and query fields retain their own editing ownership.
  Windows/Linux native shortcut behavior remains a separate gate.
  Save Current Recorded Macro names a
  copy in a separate private `named-macros.json`; the saved list is runnable
  from Macro or the repeat dialog. Repeat accepts 1-9999 runs or until EOF,
  with a progress guard and no partial commit on failure. Playback uses the
  current caret and commits its actual primitive edits as one undo unit, keeping
  untouched five-style/Find Mark ranges, clone bookmarks and directional Redo
  selection. Steps may edit text inserted by earlier steps; equal-net-text edits
  retain their actual mutations and one Undo unit in document save-point mode. A
  1048576-primitive budget prevents repeated small-buffer work from accumulating
  an unbounded temporary log. Source composition/restrictions and failed
  count/size/EOF/budget checks commit nothing. The latest recording remains in
  `macro.json`; switching documents or using unsupported navigation/search
  cancels recording instead of saving an inaccurate macro. Shortcut assignment
  and complete Scintilla action recording are not yet compatible.
- Language provides 47 selectable modes: 46 mapped to original static language
  IDs with actual Chroma lexers, plus the existing Markdown mode. The compact
  menu scrolls to every entry; manual choices and Normal Text overrides persist
  in private session v10 and dirty recovery v8, plus supported Notepad++ XML sessions. Unknown
  NPP language names fail preflight. Chroma's per-line lexer rendering is not
  an exact Scintilla or multiline-state reproduction; user-defined languages
  and unregistered original lexers remain missing.
- View > Fold All, Unfold All, current level and levels 1-8 use lexer-derived
  brace folds for common C-like/JSON modes and indentation folds for Python/YAML.
  Gutter chevrons toggle nested blocks; visible row navigation, scroll, search
  and selection reveal use Viva's fold-aware layout without editing file bytes.
  Private session v10 stores fold heads with a disk digest, dirty recovery v8
  stores them with buffer text, and supported NPP XML `<Fold>` rows round-trip.
  Other language fold grammars and exact Scintilla levels are still missing.
- View > Document Map opens a right dock sampled from Viva's visible editor
  rows, including folds and soft wraps. Click or drag its viewport to scroll
  the active pane without moving the caret or editing text; wheel input follows
  that pane. It can coexist with the left workspace dock, and private session
  v10 restores visibility. Original Scintilla map glyphs, docking resize and
  Windows pixels remain unverified.
- View > Function List opens a filtered left dock for the active buffer. Go
  uses the standard AST parser; Python/Rust use Chroma definition tokens.
  Click or press Enter to reveal a symbol without changing file bytes or undo.
  The Function List and Folder as Workspace share dock tabs when both are open;
  private session v10 restores their visibility and active tab. Other language
  parsers, original nested tree grouping and Windows visual parity remain open.
- View > Document List groups open tabs by editor view, shows live modified and
  pinned status, activates tabs, and closes through the normal dirty-save
  confirmation. Closing a shared clone detaches only that view. It shares the
  left dock tabs with Workspace and Function List, scrolls long lists, and
  restores visibility and active dock from private session v11. Original
  columns, context actions, sorting and Windows visual parity remain open.
- View > Synchronize Vertical/Horizontal Scrolling keeps the existing relative
  view offsets when switched on. Either view's wheel, scrollbars and Document
  Map can drive the other view independently by axis; the commands leave
  text, caret and undo untouched. Viva's horizontal offset API merged in
  [PR #146](https://github.com/vivaui/viva/pull/146). Exact
  Scintilla scroll notifications and Windows runtime remain unverified.
- View > Always on Top toggles the main window above ordinary windows without
  modifying text or saved preferences. The menu check follows a successful
  native request; GTK Wayland reports unsupported. Original Windows pixels
  and physical Windows/Linux window-manager behavior remain unverified.
- View > Summary... shows the active editor view's saved path and available
  file times, character/word/line counts, UTF-8 bytes with preserved physical
  line endings, and the size and number of selected areas, including multiple
  stream selections and rectangular selections. It is a read-only snapshot
  with Escape/OK dismissal. Word counts follow the Notepad++ Summary
  separator rule. Exact Scintilla code-page and character counting remain
  unverified.
- View > Monitoring (tail -f) watches a saved, clean document and reloads
  appends or replacements without changing its encoding. Monitored tabs are
  read-only, follow the file's end and stop on persistent deletion or rename;
  invalid external bytes and unexpected local edits keep the prior text.
  Closing a tab or window releases its watcher. As in the original NPP XML,
  monitoring is runtime-only; toolbar icons and native Windows event behavior
  remain unverified.
- Window > Sort By offers all ten original sorting directions for the active
  view, with stable natural names, pin-safe tab groups and metadata preflight.
  Window > Windows... can activate, save, multi-select and close tabs through
  the existing dirty confirmation, or sort tabs by its chosen column. The
  Window menu remains reachable from a compact top-bar overflow control.
  Recent Window replaces the original disabled placeholder with up to 40
  current-view tabs in tab order, marking the current and dirty ones. Exact
  Windows label formatting, collation and dialog pixels remain unverified.
- Find/replace with case, whole-word and regex options, capture expansion,
  wrapped next/previous and highlighted matches. Find All in Current Document
  lists up to 2000 matches from the live buffer, including unsaved text; result
  clicks verify the original span and return to the owning view. Replace all is
  one undo unit. Current-document Replace All uses forward individual targets,
  preserving untouched indicator gaps and mapping selection, rather than
  replacing the full buffer; adjacent/equal targets and EOL range inheritance
  are compared with original Scintilla sequences. Open-document replacement
  also commits copied target batches after validating every independent source.
  Search > Mark... opens a modeless panel with Normal, Extended
  and Regex modes, selection/wrap/purge/bookmark options, and Mark All, Clear
  and Copy Marked Text actions. Persistent marks remain after closing it;
  primitive edits shift or extend their ranges under source indicator rules. Search > Jump Up/Down > Find Mark
  Style selects the previous/next marked occurrence with wrap. Search > Find All in Open Documents gathers matches across
  every live tab, including unsaved buffers, with the same global 2000-result
  cap. Closing a source removes its results while leaving other tabs' results
  navigable. F4 and Shift+F4 cycle the result list; a clicked result becomes
  the navigation anchor, and stale or missing sources do not advance it. F7
  and Search > Search Results Window toggle focus between the result pane and
  editor; list keys highlight a result and Enter opens it.
- Search > Style All Occurrences of Token and Style One Token add independent
  ranges in five source-derived colors. Clear Style, Copy Styled Text and
  Jump Up/Down address each style separately, with all-style clear/copy actions.
  The original `100/255` fill alpha is used; Find Mark keeps its own red layer.
  Their primitive insert/delete/replacement rules match 1080 cases generated
  from the pinned Scintilla Decoration implementation. Native typing, IME,
  disjoint column/multi edits and Undo/Redo consume actual byte-edit cursors;
  an expired journal or replaced editor clears uncertain ranges. Remaining
  whole-block command mappings, same-state Windows pixels and exact round-box
  outline metrics remain unverified.
- Search > Selected Text Search uses the selection or current word for
  Select and Find Next/Previous (Primary+F3 / Primary+Shift+F3). The volatile
  pair (Primary+Alt+F3, with Shift for previous) searches the selection
  case-insensitively with wrapping without changing Find What or its options.
  A selection above the original default 1024-character Find threshold is
  refused by Select and Find. The original menu placement and configurable
  search settings are not yet mirrored.
- Search > Incremental Search (Primary+Alt+I) opens a separate bottom find
  band. Typing searches immediately; arrows, Enter/Shift+Enter and F3 navigate
  in either direction. Match case, Highlight all and Count are independent
  toggles; Escape closes the band while retaining the selected match. Its
  query never overwrites ordinary Find What. The original rebar and indicator
  styling are not yet pixel-matched.
- Search > Bookmark toggles, navigates, clears and inverses per-document
  bookmarks, with Ctrl+F2, F2 and Shift+F2 shortcuts. Cut/copy bookmarked
  lines, paste to replace each marked line, and remove marked or unmarked lines
  work as one undo unit for edits. The marker lane stays visible with line
  numbers hidden and follows wrapped and scrolled logical lines. NPP XML
  sessions round-trip `<Mark line="..."/>` entries, rejecting invalid marks
  before import. Current markers follow actual per-line merges/insertion through input,
  line rearrangement, macro replay, line sorting/removal and in-memory current,
  all and open-tab replacements. Deleting LF can merge markers onto a surviving
  line, and Undo does not recreate deleted sets. Marked-line deletion explicitly
  removes the selected marks before reverse-order line targets; paste replaces
  only each marked line's content, retaining actual insertion-line effects.
  Missing history is reported, retained and blocks marker-dependent commands
  and NPP export until clear/reload. Disk and search-replace reloads retain
  identifiable marked lines across new document objects; uncertain repeated
  lines are dropped rather than transferred to a different line. Exact
  Notepad++ reload marker behavior still needs runtime comparison.
- Search > Go to Matching Brace and Select All In-between {} [] or () use
  primary+B and primary+Alt+B. They prefer a bracket before the caret, include
  both brackets when selecting, and leave unmatched text alone.
- Find in Files scans a chosen directory with glob and recursive options in a
  cancellable background task. Select Unicode (auto), GB18030 or Windows-1252
  explicitly for the directory. Matches show path, line, column and context;
  clicking one opens the file and selects the match. Oversized, unreadable and
  unsupported files are counted as skipped; the default limits are 8 MiB per
  file and 2000 matches. Replace in Files first previews the exact files and
  counts, then requires explicit confirmation. It rejects dirty open tabs and
  tabs opened with a different encoding, validates replacement text is
  representable before confirmation, rejects files changed after preview,
  preserves encoding and line endings, and reports
  partial completion if a later file fails. Cross-file undo is not available.
- Search > Replace All in Open Documents applies the current query and replacement
  to unique live documents, including unsaved ones and shared clones. It prepares
  every result and checks legacy encoding, editor constraints, copied target
  metadata and source fingerprints before editing any tab. Untouched styles,
  Find Mark ranges and bookmarks survive; each changed source has one undo unit.
  A later preflight refusal leaves every tab and its Undo/Redo/journal unchanged,
  and can be retried after the refusal is resolved. Distinct documents sharing
  one editor are refused rather than committed ambiguously.
  The operation does not write disk files, and a visible `Unsaved` status reminds
  the user to save the changed tabs.
- UTF-8 (with optional BOM), UTF-16 LE/BE with BOM, and 47 registered explicit
  legacy character sets round-trip LF/CRLF/CR-only with atomic save. Use File >
  Open as Encoding... for legacy input; Encoding > Character sets changes the
  active document's pending save format. The 47 include GB18030 plus all 46
  original-menu character sets; six OEM pages use pinned Windows/Unicode
  single-byte tables where `x/text` has no codec.
  Invalid or non-round-trippable bytes and unmappable edits are rejected rather
  than replaced. Private sessions, recent/closed history, recovery and NPP XML
  sessions retain registered code pages. Binary files are rejected. Mixed
  LF/CRLF/CR files retain their physical separators across ordinary edits,
  Save Copy, recovery and directory replacement; new breaks use the first
  file break. If an edit makes a standalone CR immediately precede a standalone
  LF, Save and Save Copy refuse to write until the separators can be preserved.
  Explicit Edit > EOL Conversion makes the separators uniform, including
  Macintosh (CR), and Show End of Line labels each real break. Complex line
  reordering still needs original Scintilla comparison. Save refuses to
  overwrite a file modified externally after opening;
  Reload from Disk is available when the on-disk version should replace it.
- Compact menu, Figma-exported Lucide icons compiled to Viva's vector API,
  16px graphite toolbar glyphs centered in 26px controls with 4px gaps,
  a restrained teal Find selection, tabs, a monospaced code editor, Chroma
  language highlighting with per-tab Language selection, search bar, and
  status line. The application icon uses the approved mint V on a dark tile,
  retaining the original transparent 1024/128/64/32 PNG assets. Run
  `go run ./tools/icon` after changing exported SVG sources; this regenerates
  only the compiled controls and never overwrites the application PNGs.
- View > Show Symbol toggles space/tab marks, LF/CRLF/CR markers, 49 source
  non-printing abbreviations and 64 control/Unicode EOL abbreviations across
  both panes. Show All Characters switches the four independent flags together.
  Settings > Preferences > Symbols offers original, abbreviation and `U+`
  codepoint modes, with optional control-character coupling. The pinned
  source table is generated by `tools/nppsymbols` and checked entry by entry.
  Visible labels use inverted rounded BLOB fills; source bytes, selection and
  Undo stay intact. Exact Scintilla padding/font pixels and original Windows
  screenshots remain unverified.
- Show Indent Guide starts enabled as in Notepad++, is shared by both panes and
  uses a persisted toggle. Leading space/tab stops and blank-line neighbors
  paint bounded dotted guides without changing text or caret geometry. Exact
  active-guide styling and original Windows pixels are still unverified.
- Show Wrap Symbol starts off and adds a muted end marker only where Word Wrap
  creates a soft row. It reserves a cell, keeps hard EOLs unmarked and retains
  the setting across view changes and restarts; original Scintilla glyph pixels
  remain unverified.
- View > Tab can jump to tabs 1-9, navigate first/last/next/previous with
  wrapping, and move a tab within its view and pinned group. A shared clone
  owned by the other view is not reordered, to avoid moving that view's tabs;
  independent clone ordering remains incomplete.
- View > Tab can also apply five document colors or remove one. Clone views
  share the same color; private session v12, dirty recovery v9 and supported
  Notepad++ XML `tabColourId` round-trip it. The default classic-light hues
  are approximated; Windows theme pixels have not been accepted.
- View > Focus on Another View transfers keyboard focus between split editors
  without changing either document or its selection. With one editor it
  refocuses that editor. The last View menu item remains visible at 640x400.
- View > Hide Lines hides selected interior lines without editing text. The
  first and last document lines stay visible; adjacent sections merge and
  gutter chevrons restore hidden rows. Clone views share the presentation,
  and edits clear stale hidden-line locations. Scintilla marker details and
  original Windows pixels remain unverified.
- View > Toggle Full Screen Mode uses Viva's native window transition and
  restores the previous window frame on exit. The in-app menu and toolbar
  hide while fullscreen; the top-right restore control or Escape returns to
  the window. The original Windows restore-button pixels remain unverified.
- The View menu scrolls on compact windows, keeping every command at a
  readable height with wheel and up/down controls; desktop keeps the full
  menu visible.
- Main and sub editors can zoom independently. View > Synchronize Zoom Across
  Views copies the active view's size to the other and keeps subsequent zoom
  actions aligned; settings v2 stores both sizes and migrates v1's shared
  size. Exact Scintilla zoom increments and Windows font pixels remain open.

## Parity status

Notepad++ v8.9.8.1 is the version target. The app temporarily pins a published
Viva fork while upstream PR #147 remains open; #139, #142, #143 and #146 have
merged. Native window chrome, DPI, font metrics, toolbar imagery and Scintilla's
Document Map still need same-state Windows comparison. Six OEM code pages,
automatic detection,
exact mixed-line reorder and locale-specific operations, cross-file undo,
macro shortcut assignment, advanced preferences, printing and many original
commands remain incomplete. Plugin ABI is deferred. The current build is not
pixel-identical or functionally one-to-one with Notepad++.

The macOS host used for verification has no usable Wine or Windows VM.
Actual pixel-diff acceptance requires an original Notepad++ screenshot at the
same version, OS, DPI, dimensions and font configuration. Detailed verification
records, command inventories and comparison captures remain local.
Viva's mark source and the icon composition are
credited in `assets/ATTRIBUTION.md`; language highlighting uses Chroma (MIT).

The comparison tool requires equal-sized PNGs and never resizes a reference:

```sh
go run ./tools/compare \
  -reference /path/to/notepad-plus-plus.png \
  -actual /path/to/vivapad.png \
  -diff /path/to/difference.png \
  -tolerance 8 -max-changed-percent 1
```

It prints JSON metrics and returns a nonzero status when the configured
changed-pixel percentage is exceeded. A passing comparison is only meaningful
when both captures use the same OS, viewport, DPI, font, document and state.

Verification on 2026-11-07: `go test ./... -count=1`, `go test -race ./...`,
`go vet ./...`, native macOS build/start and `.app` packaging with a 1024px
icon, Windows and Linux cross-builds, and headless `layout` at 1280x800 and
640x400. Local VivaPad captures are not Notepad++ reference screenshots.
Windows and Linux binaries have not been run on those
systems; physical IME and pixel parity remain unverified.
