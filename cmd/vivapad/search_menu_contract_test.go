package main

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func searchCommandIndex(t *testing.T, command string) int {
	t.Helper()
	index := slices.Index(menuCommands[2], command)
	if index < 0 {
		t.Fatalf("Search command %q is missing", command)
	}
	return index
}

func searchCommandID(t *testing.T, command string) gfx.ID {
	t.Helper()
	return menuItemScope.At(searchCommandIndex(t, command))
}

func TestSearchMenuFollowsPinnedOriginalOrder(t *testing.T) {
	// Notepad_plus.rc dd40fe4, lines 650-747. The existing Spec groups the
	// original four selection-driven entries under Selected Text Search.
	want := []string{
		"Find...", "Find in Files...", "Find Next", "Find Previous",
		"Selected Text Search", "Replace...", "Incremental Search",
		"Search Results Window", "Next Search Result", "Previous Search Result",
		"Go to Line...", "Go to Matching Brace", "Select All In-between {} [] or ()",
		"Mark...", "Change History", "Style All Occurrences of Token", "Style One Token",
		"Clear Style", "Jump Up", "Jump Down", "Copy Styled Text", "Bookmark",
		"Find characters in range...",
	}
	commands := menuCommands[2]
	if len(commands) < len(want) || !slices.Equal(commands[:len(want)], want) {
		t.Fatalf("Search primary order = %q, want %q", commands, want)
	}
	aliases := []string{
		"Replace in Files...", "Find All in Current Document", "Find All in Open Documents",
		"Replace All in Open Documents", "Clear Marks", "Next Change", "Previous Change", "Clear Change History",
	}
	if !slices.Equal(commands[len(want):], aliases) {
		t.Fatalf("Search compatibility commands = %q, want %q", commands[len(want):], aliases)
	}
	seen := make(map[string]bool)
	for _, command := range commands {
		if seen[command] {
			t.Fatalf("Search command %q appears more than once", command)
		}
		seen[command] = true
	}
}

func TestSearchSubmenusFollowTheirPaintedParentRows(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 1000}, {"compact", 640, 400}} {
		for _, popup := range []struct {
			command string
			first   gfx.ID
			rows    int
		}{
			{"Change History", changeHistoryScope.At(0), 3},
			{"Bookmark", bookmarkScope.At(0), 10},
			{"Selected Text Search", selectedSearchScope.At(0), 4},
			{"Style All Occurrences of Token", styleMenuScope.At(0), 5},
			{"Style One Token", styleMenuScope.At(0), 5},
			{"Clear Style", styleMenuScope.At(0), 6},
			{"Copy Styled Text", styleMenuScope.At(0), 7},
			{"Jump Up", markJumpScope.At(0), 6},
			{"Jump Down", markJumpScope.At(0), 6},
		} {
			t.Run(size.name+"/"+popup.command, func(t *testing.T) {
				state, driver := startDriver(t)
				if err := driver.Resize(size.width, size.height); err != nil {
					t.Fatal(err)
				}
				doc := state.space.Current()
				before, selection, dirty := doc.Editor.String(), doc.Editor.Selection(), doc.Dirty()
				clickID(t, driver, menuScope.At(2))
				index := searchCommandIndex(t, popup.command)
				if size.name == "compact" {
					for state.searchMenuOffset < min(index, len(menuCommands[2])-state.searchMenuVisible) {
						clickID(t, driver, searchMenuNextID)
					}
				}
				parent := syncScrollHit(t, driver, menuItemScope.At(index))
				clickID(t, driver, menuItemScope.At(index))
				child := syncScrollHit(t, driver, popup.first)
				height := float32(popup.rows*27 + 8)
				wantY := min(parent.Y, max(menuHeight, float32(size.height)-statusHeight-height-4)) + 4
				if math.Abs(float64(child.Y-wantY)) > 0.01 {
					t.Fatalf("submenu first row y=%v, want %v from parent %+v", child.Y, wantY, parent)
				}
				if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
					t.Fatalf("Search submenu layout: %+v", problems)
				}
				if doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Dirty() != dirty {
					t.Fatal("opening a Search submenu changed the document")
				}
			})
		}
	}
}

func TestChangeHistorySubmenuKeyboardNavigation(t *testing.T) {
	for _, pane := range []string{"left", "right"} {
		t.Run(pane, func(t *testing.T) {
			state, driver, editor := submenuDocument(t)
			focus := editorID
			if pane == "right" {
				state.command("Clone to Other View")
				state.activatePane(1)
				state.invalidator.Invalidate()
				if err := driver.Pump(); err != nil {
					t.Fatal(err)
				}
				focus = rightEditorID
				clickID(t, driver, focus)
			}
			clickID(t, driver, menuScope.At(2))
			clickID(t, driver, searchCommandID(t, "Change History"))
			if err := driver.Key(gfx.KeyDown, 0); err != nil {
				t.Fatal(err)
			}
			if err := driver.Key(gfx.KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(editor.String()[:editor.ByteOffset(editor.Caret())], "\n"); got != 1 {
				t.Fatalf("keyboard Next Change landed on line %d, want 1", got)
			}
			if state.menuOpen >= 0 || state.changeHistoryMenuOpen || driver.Snapshot().Focused != focus {
				t.Fatalf("keyboard command menu=%d child=%v focus=%v, requested=%v armed=%v", state.menuOpen, state.changeHistoryMenuOpen, driver.Snapshot().Focused, state.focusViewRequested, state.focusViewArmed)
			}
			before, offset := editor.String(), editor.ByteOffset(editor.Caret())
			if err := driver.Type("Q"); err != nil {
				t.Fatal(err)
			}
			if editor.String() != before[:offset]+"Q"+before[offset:] {
				t.Fatal("immediate typing after the keyboard command missed its target")
			}
		})
	}
}

func TestChangeHistorySubmenuKeyboardUsesActiveDistinctDocument(t *testing.T) {
	for _, pane := range []int{0, 1} {
		t.Run([]string{"left", "right"}[pane], func(t *testing.T) {
			state, driver, editor := submenuDocument(t)
			target := state.space.Current()
			other := state.space.NewDocument()
			other.Editor.Insert("other document stays unchanged")
			other.Editor.SetSelection(text.Range{Start: 2, End: 8})
			state.splitEnabled = true
			state.paneDocs[pane], state.paneDocs[1-pane] = target, other
			state.paneOwner[target], state.paneOwner[other] = pane, 1-pane
			state.activatePane(pane)
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			otherBefore, otherSelection := other.Editor.String(), other.Editor.Selection()
			clickID(t, driver, menuScope.At(2))
			clickID(t, driver, searchCommandID(t, "Change History"))
			for _, key := range []gfx.Key{gfx.KeyDown, gfx.KeyEnter} {
				if err := driver.Key(key, 0); err != nil {
					t.Fatal(err)
				}
			}
			if got := strings.Count(editor.String()[:editor.ByteOffset(editor.Caret())], "\n"); got != 1 {
				t.Fatalf("active pane %d navigated to line %d, want 1", pane, got)
			}
			before, offset := editor.String(), editor.ByteOffset(editor.Caret())
			if err := driver.Type("Q"); err != nil {
				t.Fatal(err)
			}
			if editor.String() != before[:offset]+"Q"+before[offset:] || state.space.Current() != target || state.activePane != pane {
				t.Fatal("keyboard command or immediate input missed the active document")
			}
			if other.Editor.String() != otherBefore || other.Editor.Selection() != otherSelection {
				t.Fatal("keyboard command changed the inactive document")
			}
		})
	}
}

func TestSearchParentScrollDismissesOpenSubmenu(t *testing.T) {
	state, driver, _ := submenuDocument(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Change History"))
	panel := syncScrollHit(t, driver, searchMenuPanelID)
	if err := driver.Scroll(gfx.ScrollEvent{X: panel.X + 2, Y: panel.Y + 40, DeltaY: 120}); err != nil {
		t.Fatal(err)
	}
	if state.menuOpen != 2 || state.changeHistoryMenuOpen {
		t.Fatal("scrolling the Search parent retained its old submenu")
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("scrolled Search layout: %+v", problems)
	}
}

func TestSearchSubmenuCommandRevealsPreviouslyScrolledParent(t *testing.T) {
	state, driver, _ := submenuDocument(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	for state.searchMenuOffset < len(menuCommands[2])-state.searchMenuVisible {
		clickID(t, driver, searchMenuNextID)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	state.command("Change History")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.entryVisible("Change History") {
		t.Fatal("programmatic submenu open did not reveal its parent")
	}
	syncScrollHit(t, driver, searchCommandID(t, "Change History"))
	syncScrollHit(t, driver, changeHistoryScope.At(0))
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("revealed Search submenu layout: %+v", problems)
	}
}

func TestScrolledMenuLeafExecutesOnlyOnce(t *testing.T) {
	for _, test := range []struct {
		name, command, initial, want string
		menu, caret, wantCaret       int
		scroll                       gfx.ID
	}{
		{"Search brace", "Go to Matching Brace", "{\u732b}", "{\u732b}", 2, 1, 2, searchMenuNextID},
		{"Edit duplicate", "Duplicate Line", "x\n", "x\nx\n", 1, 0, -1, editMenuNextID},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert(test.initial)
			doc.Editor.MoveTo(test.caret, false)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(test.menu))
			clickID(t, driver, test.scroll)
			index := slices.Index(menuCommands[test.menu], test.command)
			clickID(t, driver, menuItemScope.At(index))
			if doc.Editor.String() != test.want || test.wantCaret >= 0 && doc.Editor.Caret() != test.wantCaret {
				t.Fatalf("one click of %q produced %q/caret %d, want %q/caret %d", test.command, doc.Editor.String(), doc.Editor.Caret(), test.want, test.wantCaret)
			}
		})
	}
}
