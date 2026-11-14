# Change-History Metadata

This package adapts the metadata algorithm from Scintilla ChangeHistory at
Notepad++ v8.9.8.1, commit `dd40fe45598791e9aa38f3aeb200e10fb3d6338a`.
Source ChangeHistory copyright: 2022 Neil Hodgson <neilh@scintilla.org>.
The source permission/warranty notice is retained in `SCINTILLA-LICENSE.txt`.

The oracle observes the actual original CellBuffer calls and queries original
ChangeHistory. It does not replace the original UndoHistory decisions with a
second implementation. Every translation unit using CellBuffer sees the same
observing subclass; original ChangeHistory itself is compiled unchanged.

The Go representation uses edition runs, sparse deletion sets and counted
history spans, not text copies or an integer per byte. Its event flags must
come from real history/save/detach state. Passing these metadata fixtures alone
does not provide the editor lifecycle, history budget, margin/indicator UI,
navigation/clear commands, native behavior or pixel acceptance.

## Reference Boundary

The unchanged pinned original asserts after 2293 complete states at
`script-18/57`; the plain original (no observer) reproduces the same assertion.
AddressSanitizer on the original release build reports a heap-buffer-overflow
in `ChangeLog::PopDeletion`. The loop passes its total span count to PopSpan
after already restoring part of that count. A compressed preceding deletion
can therefore be consumed past this action's boundary.

`scintilla-change-history.tsv` retains only the flushed, unchanged-original
prefix: 2293 states / 140312 queries. No guessed original rows fill the gap.
The separate `scintilla-change-history-span-fixed.tsv` uses the checked-in
one-line experiment changing `PopSpan(inserts)` to `PopSpan(inserts - i)`.
It contains 8299 states / 515916 queries and is explicitly **not** acceptance
against an unmodified original. Debug assertions and AddressSanitizer pass
that experiment, and two generations compare byte-identically. This Go
package uses the remaining-span correction and a focused regression for it;
it does not intentionally reproduce the original memory error.

Restoration preflight also validates the compressed spans and deletion
editions before mutating either log. Malformed reversion events that cannot
restore a complete deletion return an error without consuming metadata.

Reproduction records and the still-open integration contract are retained
locally. No upstream Notepad++ fix or issue
has been published by this task.

## Line Projection And Navigation

`Tracker.LineMask(start, end, next)` follows original `Document::GetMark` over
caller-supplied UTF-8 line bounds. It returns the four low edition bits and
does not mix in bookmarks. Inserted EOL bytes belong to the preceding line;
deletion points stop inclusively at the content end, including an empty last
line's EOF. Invalid bounds return an error without modifying the tracker.

`ChangedLine(current, lineCount, previous, marked)` follows the original
`changedHistoryGoTo` block skipping and asymmetric wrap. It returns `(-1, false)`
for no change or invalid arguments. The callback queries current change-history
markers; these helpers neither infer history from dirty state nor change an
editor's caret, selection, document or Undo stack.

The pinned original methods are compiled by the source-checked oracle in
`tools/nppindicators/build_change_navigation_oracle.sh`. The unchanged-original
prefix covers 2293 states / 9170 line masks. The separately labeled span
experiment covers 8299 states / 32912 line masks. Every pattern and cursor for
1–9 lines covers 16388 original navigation outcomes. The adapter supplies
normalized-LF boundaries and original metadata query rows, not native Scintilla
Document layout or GUI messages. The application history producer, UI commands,
settings, margins and native acceptance are still outstanding.
