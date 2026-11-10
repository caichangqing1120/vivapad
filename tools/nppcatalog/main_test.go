package main

import (
	"strings"
	"testing"
)

func TestParseMainMenuKeepsNestedPathsAndIgnoresOtherResources(t *testing.T) {
	source := `IDR_M30_MENU MENU
BEGIN
    POPUP "&Edit"
    BEGIN
        MENUITEM "&Undo", IDM_EDIT_UNDO
        MENUITEM SEPARATOR
        POPUP "Cop&y && More"
        BEGIN
            MENUITEM "Copy All, Now", IDM_EDIT_COPY_ALL
        END
    END
    POPUP "&Search"
    BEGIN
        MENUITEM "&Find...", IDM_SEARCH_FIND
    END
    MENUITEM "＋", IDM_FILE_NEW, HELP
END
IDR_OTHER_MENU MENU
BEGIN
    MENUITEM "Ignored", IDM_OTHER
END`
	commands, err := parseMainMenu(strings.NewReader(source))
	if err != nil || len(commands) != 3 {
		t.Fatalf("parsed commands = %+v, err=%v", commands, err)
	}
	if commands[0] != (command{Menu: "Edit", Path: "Edit / Undo", ID: "IDM_EDIT_UNDO"}) ||
		commands[1] != (command{Menu: "Edit", Path: "Edit / Copy & More / Copy All, Now", ID: "IDM_EDIT_COPY_ALL"}) ||
		commands[2] != (command{Menu: "Search", Path: "Search / Find...", ID: "IDM_SEARCH_FIND"}) {
		t.Fatalf("menu hierarchy = %+v", commands)
	}
}

func TestParseMainMenuRejectsMalformedResource(t *testing.T) {
	for _, source := range []string{
		`IDR_OTHER_MENU MENU`,
		`IDR_M30_MENU MENU
BEGIN
POPUP "Edit"
BEGIN
MENUITEM "Undo", IDM_EDIT_UNDO`,
		`IDR_M30_MENU MENU
BEGIN
POPUP "Edit"
BEGIN
MENUITEM "Untyped"` + "\nEND\nEND",
		"IDR_M30_MENU MENU\nBEGIN\nMENUITEM \"Unknown\", IDM_UNKNOWN\nEND",
	} {
		if _, err := parseMainMenu(strings.NewReader(source)); err == nil {
			t.Fatalf("malformed menu was accepted: %q", source)
		}
	}
}

func TestCatalogRequiresExplicitEvidenceForClaimedCoverage(t *testing.T) {
	commands := []command{{Menu: "Edit", Path: "Edit / Undo", ID: "IDM_EDIT_UNDO"}, {Menu: "Search", Path: "Search / Find", ID: "IDM_SEARCH_FIND"}}
	coverage, err := parseCoverage(strings.NewReader("id\tstatus\taction\tevidence\nIDM_EDIT_UNDO\tLocal\tUndo\tcmd/vivapad/shell_test.go\n"), commands)
	if err != nil {
		t.Fatal(err)
	}
	report := renderCatalog(commands, coverage, "Notepad++ test source")
	if !strings.Contains(report, "| Edit / Undo | `IDM_EDIT_UNDO` | Local | Undo | `cmd/vivapad/shell_test.go` |") ||
		!strings.Contains(report, "| Search / Find | `IDM_SEARCH_FIND` | Unreviewed |  |  |") ||
		!strings.Contains(report, "Edit: 1 local") || !strings.Contains(report, "Search: 0 local, 0 partial, 0 missing, 1 unreviewed") {
		t.Fatalf("catalog report = %q", report)
	}
	for _, invalid := range []string{
		"id\tstatus\taction\tevidence\nIDM_SEARCH_FIND\tLocal\tFind\t\n",
		"id\tstatus\taction\tevidence\nIDM_UNKNOWN\tLocal\tX\ttest.go\n",
		"id\tstatus\taction\tevidence\nIDM_EDIT_UNDO\tDone\tUndo\ttest.go\n",
	} {
		if _, err := parseCoverage(strings.NewReader(invalid), commands); err == nil {
			t.Fatalf("invalid coverage accepted: %q", invalid)
		}
	}
}

func TestSelectMenusPreservesSourceOrderAndRejectsUnknownOrRepeatedNames(t *testing.T) {
	commands := []command{
		{Menu: "File", ID: "ID_FILE"}, {Menu: "Edit", ID: "ID_EDIT"},
		{Menu: "Search", ID: "ID_SEARCH"}, {Menu: "View", ID: "ID_VIEW"},
		{Menu: "View", ID: "ID_VIEW_MORE"}, {Menu: "?", ID: "ID_HELP"},
	}
	for _, test := range []struct{ selection, want string }{
		{"all", "File,Edit,Search,View,?"},
		{"View,Edit,?", "Edit,View,?"},
		{"Edit,Search", "Edit,Search"},
	} {
		menus, err := selectMenus(commands, test.selection)
		if err != nil || strings.Join(menus, ",") != test.want {
			t.Fatalf("selection %q = %v, %v; want %q", test.selection, menus, err, test.want)
		}
	}
	for _, selection := range []string{"", "Edit,Edit", "Missing", "View, Missing", "all,View"} {
		if menus, err := selectMenus(commands, selection); err == nil {
			t.Fatalf("invalid selection %q accepted: %v", selection, menus)
		}
	}
}

func TestSelectedCatalogEnumeratesOtherMenusWithoutImpliedCoverage(t *testing.T) {
	commands := []command{
		{Menu: "Edit", Path: "Edit / Undo", ID: "ID_UNDO"},
		{Menu: "View", Path: "View / Fold / All", ID: "ID_FOLD"},
		{Menu: "?", Path: "? / Help", ID: "ID_HELP"},
	}
	menus, err := selectMenus(commands, "View,?")
	if err != nil {
		t.Fatal(err)
	}
	report := renderCatalogForMenus(commands, nil, "pinned resource", menus)
	if strings.Contains(report, "Edit / Undo") || !strings.Contains(report, "## View (1 static commands)") ||
		!strings.Contains(report, "| View / Fold / All | `ID_FOLD` | Unreviewed |  |  |") ||
		!strings.Contains(report, "## ? (1 static commands)") ||
		!strings.Contains(report, "View: 0 local, 0 partial, 0 missing, 1 unreviewed") {
		t.Fatalf("selected catalog = %q", report)
	}
}
