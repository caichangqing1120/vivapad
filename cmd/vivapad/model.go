package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/assets"
	"vivapad/internal/datetime"
	"vivapad/internal/document"
	"vivapad/internal/external"
	"vivapad/internal/foldertree"
	"vivapad/internal/hashops"
	"vivapad/internal/macro"
	"vivapad/internal/preferences"
	"vivapad/internal/printing"
	"vivapad/internal/recent"
	"vivapad/internal/recovery"
	"vivapad/internal/runcommand"
	"vivapad/internal/searchfiles"
	"vivapad/internal/session"
	"vivapad/internal/syntax"
	"vivapad/internal/websearch"
	"vivapad/internal/workspace"
)

var (
	tree                       = gfx.Tree{}
	page                       = tree.Feature("vivapad")
	menuScope                  = tree.Sub(page, tree.Name("menus"))
	menuItemScope              = tree.Sub(page, tree.Name("menu-items"))
	fileMenuPanelID            = tree.ID(page, "file-menu-panel")
	fileMenuPreviousID         = tree.ID(page, "file-menu-previous")
	fileMenuNextID             = tree.ID(page, "file-menu-next")
	viewMenuPanelID            = tree.ID(page, "view-menu-panel")
	viewMenuPreviousID         = tree.ID(page, "view-menu-previous")
	viewMenuNextID             = tree.ID(page, "view-menu-next")
	editMenuPanelID            = tree.ID(page, "edit-menu-panel")
	editMenuPreviousID         = tree.ID(page, "edit-menu-previous")
	editMenuNextID             = tree.ID(page, "edit-menu-next")
	searchMenuPanelID          = tree.ID(page, "search-menu-panel")
	searchMenuPreviousID       = tree.ID(page, "search-menu-previous")
	searchMenuNextID           = tree.ID(page, "search-menu-next")
	hiddenMenuScope            = tree.Sub(page, tree.Name("hidden-menus"))
	menuOverflowID             = tree.ID(page, "menu-overflow")
	fullscreenRestoreID        = tree.ID(page, "fullscreen-restore")
	postItRestoreID            = tree.ID(page, "post-it-restore")
	windowSortScope            = tree.Sub(page, tree.Name("window-sort"))
	windowSortPanelID          = tree.ID(page, "window-sort-panel")
	windowRecentScope          = tree.Sub(page, tree.Name("window-recent"))
	windowRecentPanelID        = tree.ID(page, "window-recent-panel")
	windowRecentPreviousID     = tree.ID(page, "window-recent-previous")
	windowRecentNextID         = tree.ID(page, "window-recent-next")
	windowsPanelID             = tree.ID(page, "windows-panel")
	windowsRowScope            = tree.Sub(page, tree.Name("windows-rows"))
	windowsHeaderScope         = tree.Sub(page, tree.Name("windows-headers"))
	windowsSortNameID          = windowsHeaderScope.At(0)
	windowsActivateID          = tree.ID(page, "windows-activate")
	windowsSaveID              = tree.ID(page, "windows-save")
	windowsCloseID             = tree.ID(page, "windows-close")
	windowsSortTabsID          = tree.ID(page, "windows-sort-tabs")
	windowsOKID                = tree.ID(page, "windows-ok")
	windowsPreviousID          = tree.ID(page, "windows-previous")
	windowsNextID              = tree.ID(page, "windows-next")
	leftFoldScope              = tree.Sub(page, tree.Name("left-fold-markers"))
	rightFoldScope             = tree.Sub(page, tree.Name("right-fold-markers"))
	leftFoldBadgeScope         = tree.Sub(page, tree.Name("left-fold-badges"))
	rightFoldBadgeScope        = tree.Sub(page, tree.Name("right-fold-badges"))
	hideLineScope              = tree.Sub(page, tree.Name("left-hidden-line-markers"))
	rightHideLineScope         = tree.Sub(page, tree.Name("right-hidden-line-markers"))
	foldLevelScope             = tree.Sub(page, tree.Name("fold-levels"))
	documentMapID              = tree.ID(page, "document-map")
	documentMapCloseID         = tree.ID(page, "document-map-close")
	functionPanelID            = tree.ID(page, "function-list-panel")
	functionFilterID           = tree.ID(page, "function-list-filter")
	functionCloseID            = tree.ID(page, "function-list-close")
	functionSymbolScope        = tree.Sub(page, tree.Name("function-list-symbols"))
	leftDockWorkspaceID        = tree.ID(page, "left-dock-workspace")
	leftDockFunctionsID        = tree.ID(page, "left-dock-functions")
	leftDockDocumentsID        = tree.ID(page, "left-dock-documents")
	leftDockCharactersID       = tree.ID(page, "left-dock-characters")
	characterPanelID           = tree.ID(page, "character-panel")
	characterCloseID           = tree.ID(page, "character-close")
	characterPrevID            = tree.ID(page, "character-previous")
	characterNextID            = tree.ID(page, "character-next")
	characterHTMLID            = tree.ID(page, "character-html")
	characterCellScope         = tree.Sub(page, tree.Name("character-cells"))
	documentListPanelID        = tree.ID(page, "document-list-panel")
	documentListCloseID        = tree.ID(page, "document-list-close")
	documentListRowScope       = tree.Sub(page, tree.Name("document-list-rows"))
	documentListRowCloseScope  = tree.Sub(page, tree.Name("document-list-row-close"))
	documentListPreviousID     = tree.ID(page, "document-list-previous")
	documentListNextID         = tree.ID(page, "document-list-next")
	summaryCloseID             = tree.ID(page, "summary-close")
	summaryRowScope            = tree.Sub(page, tree.Name("summary-rows"))
	helpDialogID               = tree.ID(page, "help-dialog")
	helpCloseID                = tree.ID(page, "help-close")
	helpCopyID                 = tree.ID(page, "help-copy")
	helpRowScope               = tree.Sub(page, tree.Name("help-rows"))
	languagePanelID            = tree.ID(page, "language-panel")
	languagePrevID             = tree.ID(page, "language-previous")
	languageNextID             = tree.ID(page, "language-next")
	charsetScope               = tree.Sub(page, tree.Name("character-sets"))
	charsetPanelID             = tree.ID(page, "character-sets-panel")
	charsetPrevID              = tree.ID(page, "character-sets-previous")
	charsetNextID              = tree.ID(page, "character-sets-next")
	savedMacroScope            = tree.Sub(page, tree.Name("saved-macros"))
	savedMacroPanelID          = tree.ID(page, "saved-macros-panel")
	savedMacroPrevID           = tree.ID(page, "saved-macros-previous")
	savedMacroNextID           = tree.ID(page, "saved-macros-next")
	macroNameID                = tree.ID(page, "macro-name")
	macroSaveID                = tree.ID(page, "macro-save")
	macroRunID                 = tree.ID(page, "macro-run")
	macroCancelID              = tree.ID(page, "macro-cancel")
	macroCountID               = tree.ID(page, "macro-count")
	macroSourceID              = tree.ID(page, "macro-source")
	macroCountModeID           = tree.ID(page, "macro-count-mode")
	macroEOFModeID             = tree.ID(page, "macro-eof-mode")
	macroSourceScope           = tree.Sub(page, tree.Name("macro-sources"))
	runOpenID                  = tree.ID(page, "run-open")
	runMenuPanelID             = tree.ID(page, "run-menu-panel")
	runMenuPreviousID          = tree.ID(page, "run-menu-previous")
	runMenuNextID              = tree.ID(page, "run-menu-next")
	runNamedScope              = tree.Sub(page, tree.Name("run-named-commands"))
	runDialogInputID           = tree.ID(page, "run-dialog-input")
	runDialogNameID            = tree.ID(page, "run-dialog-name")
	runDialogBrowseID          = tree.ID(page, "run-dialog-browse")
	runDialogVariablesID       = tree.ID(page, "run-dialog-variables")
	runDialogExecuteID         = tree.ID(page, "run-dialog-execute")
	runDialogSaveID            = tree.ID(page, "run-dialog-save")
	runDialogCancelID          = tree.ID(page, "run-dialog-cancel")
	runVariableScope           = tree.Sub(page, tree.Name("run-variables"))
	preferencesEditingID       = tree.ID(page, "preferences-editing")
	preferencesViewID          = tree.ID(page, "preferences-view")
	preferencesPrintID         = tree.ID(page, "preferences-print")
	preferencesPrintBandsID    = tree.ID(page, "preferences-print-bands")
	preferencesPrintFontsID    = tree.ID(page, "preferences-print-fonts")
	preferencesSymbolsID       = tree.ID(page, "preferences-symbols")
	printFontSizeDecreaseScope = tree.Sub(page, tree.Name("print-font-size-decrease"))
	printFontSizeIncreaseScope = tree.Sub(page, tree.Name("print-font-size-increase"))
	printFontBoldScope         = tree.Sub(page, tree.Name("print-font-bold"))
	printFontItalicScope       = tree.Sub(page, tree.Name("print-font-italic"))
	printColorScope            = tree.Sub(page, tree.Name("print-color-mode"))
	printLineNumbersID         = tree.ID(page, "print-line-numbers")
	printFormFeedID            = tree.ID(page, "print-form-feed")
	printMarginDecrementScope  = tree.Sub(page, tree.Name("print-margin-decrement"))
	printMarginIncrementScope  = tree.Sub(page, tree.Name("print-margin-increment"))
	printBandFieldScope        = tree.Sub(page, tree.Name("print-band-fields"))
	printVariableButtonID      = tree.ID(page, "print-variable-button")
	printVariableScope         = tree.Sub(page, tree.Name("print-variables"))
	printInsertVariableID      = tree.ID(page, "print-insert-variable")
	preferencesTabScope        = tree.Sub(page, tree.Name("preferences-tab-width"))
	preferencesIndentScope     = tree.Sub(page, tree.Name("preferences-indent-mode"))
	preferencesAutoIndentID    = tree.ID(page, "preferences-auto-indent")
	preferencesPairBracketsID  = tree.ID(page, "preferences-pair-brackets")
	preferencesLineNumbersID   = tree.ID(page, "preferences-line-numbers")
	preferencesWrapID          = tree.ID(page, "preferences-word-wrap")
	preferencesWhitespaceID    = tree.ID(page, "preferences-whitespace")
	preferencesEOLID           = tree.ID(page, "preferences-eol")
	preferencesIndentGuideID   = tree.ID(page, "preferences-indent-guide")
	preferencesZoomSyncID      = tree.ID(page, "preferences-zoom-sync")
	preferencesNPCModeScope    = tree.Sub(page, tree.Name("preferences-npc-mode"))
	preferencesNPCIncludeID    = tree.ID(page, "preferences-npc-include-controls")
	preferencesCloseID         = tree.ID(page, "preferences-close")
	columnTextModeID           = tree.ID(page, "column-text-mode")
	columnNumberModeID         = tree.ID(page, "column-number-mode")
	columnTextID               = tree.ID(page, "column-text")
	columnInitialID            = tree.ID(page, "column-initial")
	columnIncrementID          = tree.ID(page, "column-increment")
	columnRepeatID             = tree.ID(page, "column-repeat")
	columnBaseScope            = tree.Sub(page, tree.Name("column-base"))
	columnLeadingScope         = tree.Sub(page, tree.Name("column-leading"))
	columnUppercaseID          = tree.ID(page, "column-uppercase")
	columnApplyID              = tree.ID(page, "column-apply")
	columnCancelID             = tree.ID(page, "column-cancel")
	byteRangeModeScope         = tree.Sub(page, tree.Name("byte-range-modes"))
	byteRangeStartID           = tree.ID(page, "byte-range-start")
	byteRangeEndID             = tree.ID(page, "byte-range-end")
	byteRangeUpID              = tree.ID(page, "byte-range-up")
	byteRangeDownID            = tree.ID(page, "byte-range-down")
	byteRangeCustomID          = byteRangeModeScope.At(2)
	byteRangeWrapID            = tree.ID(page, "byte-range-wrap")
	byteRangeFindID            = tree.ID(page, "byte-range-find")
	byteRangeCloseID           = tree.ID(page, "byte-range-close")
	byteRangePanelID           = tree.ID(page, "byte-range-panel")
	saveSessionWorkspaceID     = tree.ID(page, "save-session-workspace")
	saveSessionConfirmID       = tree.ID(page, "save-session-confirm")
	saveSessionCancelID        = tree.ID(page, "save-session-cancel")
	macroSourcePrevID          = tree.ID(page, "macro-sources-previous")
	macroSourceNextID          = tree.ID(page, "macro-sources-next")
	tabScope                   = tree.Sub(page, tree.Name("tabs"))
	tabCloseScope              = tree.Sub(page, tree.Name("tab-close"))
	tabPinScope                = tree.Sub(page, tree.Name("tab-pin"))
	leftTabScope               = tree.Sub(page, tree.Name("left-tabs"))
	rightTabScope              = tree.Sub(page, tree.Name("right-tabs"))
	leftCloseScope             = tree.Sub(page, tree.Name("left-tab-close"))
	rightCloseScope            = tree.Sub(page, tree.Name("right-tab-close"))
	leftPinScope               = tree.Sub(page, tree.Name("left-tab-pin"))
	rightPinScope              = tree.Sub(page, tree.Name("right-tab-pin"))
	leftTabsBackID             = tree.ID(page, "left-tabs-back")
	leftTabsNextID             = tree.ID(page, "left-tabs-next")
	rightTabsBackID            = tree.ID(page, "right-tabs-back")
	rightTabsNextID            = tree.ID(page, "right-tabs-next")
	tabLeftID                  = tree.ID(page, "tabs-left")
	tabRightID                 = tree.ID(page, "tabs-right")
	resultScope                = tree.Sub(page, tree.Name("file-results"))
	recentScope                = tree.Sub(page, tree.Name("recent-files"))
	lineScope                  = tree.Sub(page, tree.Name("line-operations"))
	insertDateScope            = tree.Sub(page, tree.Name("insert-date-time"))
	linePanelID                = tree.ID(page, "line-operations-panel")
	linePrevID                 = tree.ID(page, "line-operations-previous")
	lineNextID                 = tree.ID(page, "line-operations-next")
	caseScope                  = tree.Sub(page, tree.Name("convert-case"))
	commentScope               = tree.Sub(page, tree.Name("comment-uncomment"))
	eolScope                   = tree.Sub(page, tree.Name("eol-conversion"))
	symbolScope                = tree.Sub(page, tree.Name("show-symbol"))
	viewTabScope               = tree.Sub(page, tree.Name("view-tab-commands"))
	namedBrowserScope          = tree.Sub(page, tree.Name("named-browser-commands"))
	viewTabPanelID             = tree.ID(page, "view-tab-panel")
	viewTabPreviousID          = tree.ID(page, "view-tab-previous")
	viewTabNextID              = tree.ID(page, "view-tab-next")
	hashScope                  = tree.Sub(page, tree.Name("hash-tools"))
	sessionScope               = tree.Sub(page, tree.Name("session-menu"))
	bookmarkScope              = tree.Sub(page, tree.Name("bookmark-menu"))
	changeHistoryScope         = tree.Sub(page, tree.Name("change-history-menu"))
	markJumpScope              = tree.Sub(page, tree.Name("mark-jump-menu"))
	styleMenuScope             = tree.Sub(page, tree.Name("style-menu"))
	closeMultipleScope         = tree.Sub(page, tree.Name("close-multiple-menu"))
	containingFolderScope      = tree.Sub(page, tree.Name("containing-folder-menu"))
	autoCompletionScope        = tree.Sub(page, tree.Name("auto-completion-menu"))
	selectedSearchScope        = tree.Sub(page, tree.Name("selected-search-menu"))
	wordCompletionScope        = tree.Sub(page, tree.Name("word-completion"))
	calltipLabelID             = tree.ID(page, "calltip-label")
	calltipArgumentID          = tree.ID(page, "calltip-argument")
	calltipPreviousID          = tree.ID(page, "calltip-previous")
	calltipNextID              = tree.ID(page, "calltip-next")
	blankScope                 = tree.Sub(page, tree.Name("blank-operations"))
	copyPathsScope             = tree.Sub(page, tree.Name("copy-paths"))
	pasteSpecialScope          = tree.Sub(page, tree.Name("paste-special"))
	onSelectionScope           = tree.Sub(page, tree.Name("on-selection"))
	webSearchEngineScope       = tree.Sub(page, tree.Name("web-search-engines"))
	webSearchDialogID          = tree.ID(page, "web-search-dialog")
	webSearchTemplateID        = tree.ID(page, "web-search-template")
	webSearchSaveID            = tree.ID(page, "web-search-save")
	webSearchCancelID          = tree.ID(page, "web-search-cancel")
	multiSelectAllScope        = tree.Sub(page, tree.Name("multi-select-all"))
	multiSelectNextScope       = tree.Sub(page, tree.Name("multi-select-next"))
	readOnlyScope              = tree.Sub(page, tree.Name("read-only"))
	folderRowScope             = tree.Sub(page, tree.Name("folder-rows"))
	folderPanelID              = tree.ID(page, "folder-panel")
	folderAddID                = tree.ID(page, "folder-add")
	folderRefreshID            = tree.ID(page, "folder-refresh")
	folderCloseID              = tree.ID(page, "folder-close")
	recentClearID              = tree.ID(page, "recent-clear")
	recentRestoreID            = tree.ID(page, "recent-restore")

	newID                = tree.ID(page, "new")
	openID               = tree.ID(page, "open")
	saveID               = tree.ID(page, "save")
	undoID               = tree.ID(page, "undo")
	redoID               = tree.ID(page, "redo")
	cutID                = tree.ID(page, "cut")
	copyID               = tree.ID(page, "copy")
	pasteID              = tree.ID(page, "paste")
	searchID             = tree.ID(page, "search")
	editorID             = tree.ID(page, "editor")
	scrollbarID          = tree.ID(page, "editor-scrollbar")
	horizontalBarID      = tree.ID(page, "editor-horizontal-scrollbar")
	rightEditorID        = tree.ID(page, "right-editor")
	rightScrollbarID     = tree.ID(page, "right-scrollbar")
	rightHorizontalID    = tree.ID(page, "right-horizontal-scrollbar")
	leftPaneID           = tree.ID(page, "left-pane")
	rightPaneID          = tree.ID(page, "right-pane")
	splitterID           = tree.ID(page, "editor-splitter")
	queryID              = tree.ID(page, "query")
	markPanelID          = tree.ID(page, "mark-panel")
	markBookmarkID       = tree.ID(page, "mark-bookmark")
	markPurgeID          = tree.ID(page, "mark-purge")
	markInSelectionID    = tree.ID(page, "mark-in-selection")
	markWrapID           = tree.ID(page, "mark-wrap")
	markDotAllID         = tree.ID(page, "mark-dot-all")
	markModeScope        = tree.Sub(page, tree.Name("mark-mode"))
	markAllID            = tree.ID(page, "mark-all")
	markClearID          = tree.ID(page, "mark-clear")
	markCopyID           = tree.ID(page, "mark-copy")
	markCloseID          = tree.ID(page, "mark-close")
	incrementQueryID     = tree.ID(page, "increment-query")
	incrementPreviousID  = tree.ID(page, "increment-previous")
	incrementNextID      = tree.ID(page, "increment-next")
	incrementMatchCaseID = tree.ID(page, "increment-match-case")
	incrementHighlightID = tree.ID(page, "increment-highlight")
	incrementCountID     = tree.ID(page, "increment-count")
	incrementCloseID     = tree.ID(page, "increment-close")
	replacementID        = tree.ID(page, "replacement")
	findNextID           = tree.ID(page, "find-next")
	findPreviousID       = tree.ID(page, "find-previous")
	findAllCurrentID     = tree.ID(page, "find-all-current")
	replaceID            = tree.ID(page, "replace")
	replaceAllID         = tree.ID(page, "replace-all")
	replaceOpenID        = tree.ID(page, "replace-open")
	searchCloseID        = tree.ID(page, "search-close")
	matchCaseID          = tree.ID(page, "match-case")
	wholeWordID          = tree.ID(page, "whole-word")
	regexID              = tree.ID(page, "regex")
	goToFieldID          = tree.ID(page, "go-to-field")
	goToButtonID         = tree.ID(page, "go-to-button")
	goToCloseID          = tree.ID(page, "go-to-close")
	filesRootID          = tree.ID(page, "files-root")
	filesPatternID       = tree.ID(page, "files-pattern")
	filesBrowseID        = tree.ID(page, "files-browse")
	filesSearchID        = tree.ID(page, "files-search")
	filesCancelID        = tree.ID(page, "files-cancel")
	replacePreviewID     = tree.ID(page, "replace-preview")
	replaceConfirmID     = tree.ID(page, "replace-confirm")
	replaceCancelID      = tree.ID(page, "replace-cancel")
	replaceBackdropID    = tree.ID(page, "replace-backdrop")
	filesRecursiveID     = tree.ID(page, "files-recursive")
	filesUnicodeID       = tree.ID(page, "files-unicode")
	filesGB18030ID       = tree.ID(page, "files-gb18030")
	filesWindowsID       = tree.ID(page, "files-windows-1252")
	filesBackdropID      = tree.ID(page, "files-backdrop")
	resultsPaneID        = tree.ID(page, "results-pane")
	resultsCloseID       = tree.ID(page, "results-close")
	menuDismissID        = tree.ID(page, "menu-dismiss")
	dialogBackdropID     = tree.ID(page, "dialog-backdrop")
	discardID            = tree.ID(page, "discard")
	cancelCloseID        = tree.ID(page, "cancel-close")
	saveCloseID          = tree.ID(page, "save-close")
	reloadConfirmID      = tree.ID(page, "reload-confirm")
	reloadCancelID       = tree.ID(page, "reload-cancel")
	renameFieldID        = tree.ID(page, "rename-field")
	renameApplyID        = tree.ID(page, "rename-apply")
	renameConfirmID      = tree.ID(page, "rename-confirm")
	renameCancelID       = tree.ID(page, "rename-cancel")
	trashConfirmID       = tree.ID(page, "trash-confirm")
	trashCancelID        = tree.ID(page, "trash-cancel")
	dateTimeFieldID      = tree.ID(page, "date-time-format-field")
	dateTimeReverseID    = tree.ID(page, "date-time-reverse")
	dateTimeApplyID      = tree.ID(page, "date-time-apply")
	dateTimeCancelID     = tree.ID(page, "date-time-cancel")
	hashInputID          = tree.ID(page, "hash-text-input")
	hashResultID         = tree.ID(page, "hash-result")
	hashEachLineID       = tree.ID(page, "hash-each-line")
	hashBrowseID         = tree.ID(page, "hash-browse-files")
	hashCopyID           = tree.ID(page, "hash-copy")
	hashCloseID          = tree.ID(page, "hash-close")
)

type model struct {
	invalidator                   app.Invalidator
	space                         *workspace.Workspace
	editors                       map[*document.Document]*components.CodeEditor
	secondEditors                 map[*document.Document]*components.CodeEditor
	eolLabels                     [2][]string
	paneDocs                      [2]*document.Document
	paneOwner                     map[*document.Document]int
	paneTabOffset                 [2]int
	paneTabVisible                [2]int
	paneTabIndex                  [2]int
	paneTabAnchor                 [2]*document.Document
	paneTabScratch                [2][]*document.Document
	splitEnabled                  bool
	focusViewRequested            bool
	focusViewArmed                bool
	activePane                    int
	compositionOwner              compositionOwner
	compositionGeneration         uint64
	provisional                   provisionalComposition
	legacyRecovery                map[*document.Document]*recovery.Tab // nil tab means clean before preedit
	compositionFallback           map[*text.Editor]bool
	requestLinuxComposition       bool
	splitRatio                    float32
	splitDragging                 bool
	splitWidth                    float32
	languages                     map[*document.Document]string
	syntax                        syntax.Highlighter
	goHighlighters                map[*document.Document]*syntax.GoHighlighter
	json5Highlighters             map[*document.Document]*syntax.JSON5Highlighter
	innoHighlighters              map[*document.Document]*syntax.InnoHighlighter
	asn1Highlighters              map[*document.Document]*syntax.ASN1Highlighter
	kixHighlighters               map[*document.Document]*syntax.KixHighlighter
	csoundHighlighters            map[*document.Document]*syntax.CSoundHighlighter
	rebolHighlighters             map[*document.Document]*syntax.REBOLHighlighter
	avsHighlighters               map[*document.Document]*syntax.AviSynthHighlighter
	freeBasicHighlighters         map[*document.Document]*syntax.FreeBasicHighlighter
	oscriptHighlighters           map[*document.Document]*syntax.OScriptHighlighter
	pureBasicHighlighters         map[*document.Document]*syntax.PureBasicHighlighter
	gui4CliHighlighters           map[*document.Document]*syntax.Gui4CliHighlighter
	txt2tagsHighlighters          map[*document.Document]*syntax.Txt2tagsHighlighter
	hollywoodHighlighters         map[*document.Document]*syntax.HollywoodHighlighter
	visualPrologHighlighters      map[*document.Document]*syntax.VisualPrologHighlighter
	escriptHighlighters           map[*document.Document]*syntax.ESCRIPTHighlighter
	baanHighlighters              map[*document.Document]*syntax.BaanCHighlighter
	serverPageHighlighters        map[*document.Document]*syntax.ServerPageHighlighter
	resourceHighlighters          map[*document.Document]*syntax.ResourceHighlighter
	ansiBackgrounds               map[*document.Document]*syntax.ANSIBackground
	query                         *text.Editor
	incrementQuery                *text.Editor
	incrementOpen                 bool
	incrementMatchCase            bool
	incrementHighlight            bool
	incrementCount                bool
	incrementStatus               string
	incrementMatches              []text.Span
	incrementFocusPending         bool
	incrementReturnFocusRequested bool
	incrementReturnFocusArmed     bool
	incrementDocument             *document.Document
	incrementValue                string
	replacement                   *text.Editor
	goToField                     *text.Editor
	filesRoot                     *text.Editor
	filesPattern                  *text.Editor
	searchOptions                 document.SearchOptions
	marks                         searchMarkCache
	markedDocument                *document.Document
	markedValue                   string
	markedSpans                   []text.Span
	markedTracking                indicatorTracker
	styledMarks                   map[*document.Document]*styleMarkState
	markDialogOpen                bool
	markFocusPending              bool
	markBookmarkLine              bool
	markPurge                     bool
	markInSelection               bool
	markWrap                      bool
	markMode                      string
	markDotAll                    bool
	bookmarks                     map[*document.Document]*bookmarkState
	folded                        map[*document.Document]map[int]bool
	foldCache                     map[*document.Document]foldCache
	hiddenLines                   map[*document.Document][]hiddenLineRange
	hiddenValue                   map[*document.Document]string
	readOnly                      map[*document.Document]bool
	pinned                        map[*document.Document]bool
	tabColors                     map[*document.Document]uint8
	folderRoots                   []string
	folderVisible                 bool
	documentMapVisible            bool
	documentMapDragging           bool
	documentMapContent            gfx.Rect
	documentMapViewHeight         float32
	syncVertical                  bool
	syncHorizontal                bool
	syncNeedsCapture              bool
	syncScrollOffsetY             float32
	syncScrollOffsetX             float32
	syncScrollSeenY               [2]float32
	syncScrollSeenX               [2]float32
	syncScrollDocs                [2]*document.Document
	summaryOpen                   bool
	helpMode                      uint8
	helpDetails                   buildDetails
	helpCopyStatus                string
	summary                       documentSummary
	monitors                      map[*document.Document]*fileMonitor
	closeWindow                   func()
	requestFullscreen             func(bool) error
	fullscreen                    bool
	distractionFree               bool
	distractionFreeEntering       bool
	requestAlwaysOnTop            func(bool) error
	alwaysOnTop                   bool
	requestDecoration             func(bool, func(error)) error
	postIt                        postItMode
	functionListVisible           bool
	documentListVisible           bool
	documentListRows              []documentListRow
	documentListRowIDs            [2]map[*document.Document]int
	documentListNextRowID         int
	documentListOffset            int
	documentListVisibleRows       int
	documentListAnchor            *document.Document
	documentListAnchorPane        int
	documentListFocusRequested    bool
	activeLeftDock                string
	characterVisible              bool
	characterHTML                 bool
	characterOffset               int
	characterPageRows             int
	characterTable                *characterTable
	functionFilter                *text.Editor
	functionFilterFocus           bool
	functionSymbols               map[*document.Document]functionSymbolCache
	functionMatches               []int
	functionRowIDBase             int
	functionOffset                int
	functionSelected              int
	functionPageRows              int
	functionLastQuery             string
	functionPaintDoc              *document.Document
	functionPaintValue            string
	functionPaintMode             string
	folderExpanded                map[string]bool
	folderCache                   map[string][]foldertree.Entry
	folderRowsSnapshot            []folderRow
	folderSelected                string
	folderOffset                  int
	folderRowHits                 []folderRowHit
	folderNextRowID               int
	folderFocusPath               string
	folderWatcher                 *foldertree.Watcher
	folderWatching                map[string]os.FileInfo
	folderWatchUnavailable        bool
	revealPath                    func(string) error
	openConsole                   func(external.ConsoleKind, string) error
	launchSelectedFile            func(string) error
	revealSelectedPath            func(string) error
	trashPath                     func(string) error
	pendingTrash                  *trashRequest
	readOnlyDragDoc               *document.Document
	readOnlyDragPane              int
	selectAnchors                 [2]selectAnchor
	rectangles                    [2]rectangleSelection
	multiSelections               [2]multiSelectionState
	multiSelectOptions            document.SearchOptions
	multiSelectionHistory         [2][]multiSelectionTransition
	multiSelectionHistoryIndex    [2]int
	rectangleHistory              [2][]rectangleTransition
	rectangleHistoryIndex         [2]int
	rectangleDragging             bool
	rectangleDragPane             int
	searchOpen                    bool
	goToOpen                      bool
	filesDialogOpen               bool
	filesReplaceMode              bool
	filesRecursive                bool
	filesEncoding                 document.Encoding
	filesSearching                bool
	filesMessage                  string
	filesApplying                 bool
	replaceConfirmOpen            bool
	replacePlan                   *searchfiles.ReplacePlan
	resultsOpen                   bool
	fileResults                   searchfiles.Result
	resultDocument                *document.Document
	resultDocuments               []*document.Document
	resultOpenDocuments           bool
	resultQuery                   string
	resultOffset                  int
	resultSelected                int
	resultVisible                 int
	resultFocusRequested          bool
	resultFocusArmed              bool
	resultFocusMode               bool
	resultReturnToEditor          bool
	searchCancel                  context.CancelFunc
	searchReplies                 fileSearchMailbox
	menuOpen                      int
	fileMenuOffset                int
	fileMenuVisible               int
	printJob                      func(printing.Job) (int, error)
	menuOverflowOpen              bool
	menuFirstHidden               int
	windowSortMenuOpen            bool
	windowRecentOffset            int
	windowRecentVisible           int
	windowsDialogOpen             bool
	windowsDialogPane             int
	windowsRows                   []*document.Document
	windowsRowIDs                 map[*document.Document]gfx.ID
	windowsNextRowID              int
	windowsSelected               map[*document.Document]bool
	windowsFocused                *document.Document
	windowsOffset                 int
	windowsVisible                int
	windowsSort                   windowSortCommand
	windowsHasSort                bool
	windowsMessage                string
	languageOffset                int
	languageVisible               int
	recentOpen                    bool
	charsetMenuOpen               bool
	charsetOffset                 int
	charsetVisible                int
	savedMacroMenuOpen            bool
	savedMacroOffset              int
	savedMacroVisible             int
	lineMenuOpen                  bool
	insertDateOpen                bool
	hashMenuOpen                  bool
	symbolMenuOpen                bool
	viewTabMenuOpen               bool
	namedBrowserMenuOpen          bool
	openNamedBrowser              func(external.NamedBrowser, string) error
	postBrowserResult             func(func())
	namedBrowserRequest           uint64
	foldMenuOpen                  bool
	foldMenuUnfold                bool
	viewTabOffset                 int
	viewTabVisible                int
	viewMenuOffset                int
	viewMenuVisible               int
	editMenuOffset                int
	editMenuVisible               int
	searchMenuOffset              int
	searchMenuVisible             int
	hashMenuIndex                 int
	hashAlgorithm                 hashops.Algorithm
	hashDialogOpen                bool
	hashMode                      hashDialogMode
	hashInput                     *text.Editor
	hashInputLayout               components.TextareaLayout
	hashResultLayout              components.TextareaLayout
	hashResult                    string
	hashTextKey                   hashTextKey
	hashError                     string
	hashEachLine                  bool
	hashFocus                     bool
	lineMenuOffset                int
	lineMenuVisible               int
	caseMenuOpen                  bool
	commentMenuOpen               bool
	eolMenuOpen                   bool
	sessionMenuOpen               bool
	bookmarkMenuOpen              bool
	changeHistoryMenuOpen         bool
	markJumpMenuOpen              bool
	markJumpPrevious              bool
	styleMenuMode                 string
	closeMultipleMenuOpen         bool
	containingMenuOpen            bool
	autoMenuOpen                  bool
	selectedSearchMenuOpen        bool
	wordCompletion                *wordCompletionState
	calltip                       *calltipState
	wordFocusPending              bool
	blankMenuOpen                 bool
	copyPathsMenuOpen             bool
	pasteSpecialOpen              bool
	onSelectionMenuOpen           bool
	multiAllMenuOpen              bool
	multiNextMenuOpen             bool
	menuPointerShift              bool
	readOnlyMenuOpen              bool
	dateTimeDialogOpen            bool
	webSearch                     websearch.Config
	webSearchOpen                 bool
	webSearchChoice               string
	webSearchDraft                *text.Editor
	webSearchError                string
	webSearchErrorTemplate        string
	dateTimeFocus                 bool
	dateTimeDraft                 *text.Editor
	dateTimeDraftReverse          bool
	dateTimeError                 string
	dateTimeFormat                string
	dateTimeReverse               bool
	clock                         func() time.Time
	recentPath                    string
	recentEntries                 []recent.Entry
	closedPath                    string
	closedHistory                 []recent.Entry
	pendingSystemDocuments        []string
	pendingClose                  int
	batchActive                   bool
	batchQueue                    []closeTarget
	batchKeep                     *document.Document
	batchClosed                   int
	pendingReload                 int
	renameDoc                     *document.Document
	renameSource                  *document.Document
	renameField                   *text.Editor
	renamePlan                    *document.RenamePlan
	renameFocus                   bool
	notice                        string
	noChangeNoticeDoc             *document.Document
	noChangeNoticeRevision        uint64
	showLineNumbers               bool
	changeHistoryView             string
	showWhitespace                bool
	showEOL                       bool
	showIndentGuides              bool
	showWrapSymbol                bool
	showNonPrinting               bool
	showControls                  bool
	npcMode                       string
	npcIncludeControls            bool
	representations               []components.CodeRepresentation
	representationsReady          bool
	fontSize                      int
	subFontSize                   int
	zoomSync                      bool
	wrap                          bool
	tabWidth                      int
	indentMode                    string
	autoIndent                    bool
	autoClose                     bool
	preferencesPath               string
	printSettings                 printing.Settings
	preferencesOpen               bool
	preferencesPage               int
	printFields                   [6]*text.Editor
	printFieldSelected            int
	printVariableIndex            int
	printVariableOpen             bool
	columnDialogOpen              bool
	columnFocus                   bool
	columnNumber                  bool
	columnText                    *text.Editor
	columnInitial                 *text.Editor
	columnIncrement               *text.Editor
	columnRepeat                  *text.Editor
	columnBase                    int
	columnUppercase               bool
	columnLeading                 document.ColumnLeading
	columnError                   string
	byteRangeOpen                 bool
	byteRangeInitialized          bool
	byteRangeMode                 int
	byteRangeStart                *text.Editor
	byteRangeEnd                  *text.Editor
	byteRangeUp                   bool
	byteRangeWrap                 bool
	byteRangeError                string
	byteRangeFocus                bool
	saveSessionOptionsOpen        bool
	saveSessionIncludeWorkspace   bool
	saveSessionError              string
	saveSessionPicker             func(app.SaveFileOptions, app.SaveFileCheckbox) (app.SaveFileSelection, error)
	tabOffset                     int
	tabActive                     int
	tabVisible                    int
	tabHitIndices                 map[*document.Document]int
	tabNextHitIndex               int
	allowClose                    func()
	sessionPath                   string
	closingDocs                   []*document.Document
	closingTabSettings            map[*document.Document]sessionTabSettings
	closingActive                 *document.Document
	closingPaneDocs               [2]*document.Document
	closingOwners                 map[*document.Document]int
	closingSplit                  bool
	closingActivePane             int
	closingDivider                float32
	recoveryPath                  string
	recoveryWriter                *recovery.Writer
	lastRecovery                  recovery.State
	closingAccepted               bool
	macroPath                     string
	macroLast                     []macro.Step
	namedMacroPath                string
	namedMacros                   []macro.Named
	macroSaveOpen                 bool
	macroRunOpen                  bool
	macroSourceOpen               bool
	macroDialogFocus              bool
	macroDialogError              string
	macroName                     *text.Editor
	macroCount                    *text.Editor
	macroRunIndex                 int
	macroUntilEOF                 bool
	macroSourceOffset             int
	macroSourceVisible            int
	macroSteps                    []macro.Step
	macroRecording                bool
	macroSource                   *document.Document
	macroTextBytes                int
	runPath                       string
	namedRuns                     []runcommand.Named
	runLauncher                   func(runcommand.Prepared) error
	runLast                       string
	runDialogOpen                 bool
	runSaveMode                   bool
	runVariablesOpen              bool
	runDialogFocus                bool
	runDialogError                string
	runInput                      *text.Editor
	runName                       *text.Editor
	runMenuOffset                 int
	runMenuVisible                int

	// changeView caches the change-margin decorations per document.
	changeView map[*document.Document]*changeViewCache
}

type searchMarkCache struct {
	doc     *document.Document
	value   string
	query   string
	options document.SearchOptions
	spans   []text.Span
}

type fileSearchResponse struct {
	kind       fileJobKind
	generation uint64
	query      string
	result     searchfiles.Result
	plan       *searchfiles.ReplacePlan
	applied    searchfiles.ApplyResult
	err        error
}

type fileJobKind uint8

const (
	fileJobFind fileJobKind = iota
	fileJobReplacePreview
	fileJobReplaceApply
)

func newModel() *model {
	workingDirectory, _ := os.Getwd()
	space := workspace.New()
	var openConsole func(external.ConsoleKind, string) error
	if runtime.GOOS == "windows" {
		openConsole = external.OpenConsole
	}
	return &model{
		changeView: newChangeViewCache(),
		space:      space, editors: make(map[*document.Document]*components.CodeEditor),
		secondEditors: make(map[*document.Document]*components.CodeEditor), paneDocs: [2]*document.Document{space.Current()}, splitRatio: 0.5,
		paneOwner:      map[*document.Document]int{space.Current(): 0},
		languages:      make(map[*document.Document]string),
		bookmarks:      make(map[*document.Document]*bookmarkState),
		folded:         make(map[*document.Document]map[int]bool),
		foldCache:      make(map[*document.Document]foldCache),
		hiddenLines:    make(map[*document.Document][]hiddenLineRange),
		hiddenValue:    make(map[*document.Document]string),
		readOnly:       make(map[*document.Document]bool),
		pinned:         make(map[*document.Document]bool),
		tabColors:      make(map[*document.Document]uint8),
		folderExpanded: make(map[string]bool), folderCache: make(map[string][]foldertree.Entry),
		folderWatching:  make(map[string]os.FileInfo),
		functionSymbols: make(map[*document.Document]functionSymbolCache),
		functionFilter:  text.NewEditor(""), functionSelected: -1,
		revealPath:         external.Reveal,
		openNamedBrowser:   external.OpenFileInNamedBrowser,
		openConsole:        openConsole,
		launchSelectedFile: external.LaunchEditor,
		revealSelectedPath: external.RevealSelection,
		trashPath:          external.Trash,
		runLauncher:        runcommand.Launch,
		query:              text.NewEditor(""), incrementQuery: text.NewEditor(""), incrementCount: true,
		replacement: text.NewEditor(""), goToField: text.NewEditor(""),
		filesRoot: text.NewEditor(workingDirectory), filesPattern: text.NewEditor("*"),
		filesRecursive: true,
		menuOpen:       -1, pendingClose: -1, pendingReload: -1, resultSelected: -1, showLineNumbers: true, changeHistoryView: "margin", showIndentGuides: true, showControls: true, markWrap: true, markMode: "normal", fontSize: 13, subFontSize: 13,
		tabWidth: 4, indentMode: "language", autoIndent: true, autoClose: true, npcMode: "abbreviation",
		printSettings:  printing.DefaultSettings(),
		dateTimeFormat: datetime.DefaultFormat, clock: time.Now,
		lastRecovery: recovery.State{Active: -1},
	}
}

func newModelWithFiles(paths []string) *model {
	return newStartupModel(paths, "")
}

func newStartupModel(paths []string, sessionPath string) *model {
	state := newModel()
	state.sessionPath = sessionPath
	if sessionPath != "" {
		state.closedPath = filepath.Join(filepath.Dir(sessionPath), "closed.json")
		if entries, err := recent.Load(state.closedPath); err != nil {
			state.notice = "Closed files unavailable: " + err.Error()
		} else {
			state.closedHistory = entries
		}
		state.recentPath = filepath.Join(filepath.Dir(sessionPath), "recent.json")
		if entries, err := recent.Load(state.recentPath); err != nil {
			state.notice = "Recent files unavailable: " + err.Error()
		} else {
			state.recentEntries = entries
		}
		state.macroPath = filepath.Join(filepath.Dir(sessionPath), "macro.json")
		if steps, err := macro.Load(state.macroPath); err != nil {
			state.notice = "Macro unavailable: " + err.Error()
		} else {
			state.macroLast = steps
		}
		state.namedMacroPath = filepath.Join(filepath.Dir(sessionPath), "named-macros.json")
		if named, err := macro.LoadNamed(state.namedMacroPath); err != nil {
			state.notice = "Named macros unavailable: " + err.Error()
		} else {
			state.namedMacros = named
		}
		state.runPath = filepath.Join(filepath.Dir(sessionPath), "run.json")
		if named, err := runcommand.Load(state.runPath); err != nil {
			state.notice = "Run commands unavailable: " + err.Error()
		} else {
			state.namedRuns = named
		}
		state.preferencesPath = filepath.Join(filepath.Dir(sessionPath), "settings.json")
		if stored, err := preferences.Load(state.preferencesPath); err != nil {
			state.notice = "Settings not restored: " + err.Error()
		} else {
			state.applyPreferences(stored)
		}
		state.recoveryPath = filepath.Join(filepath.Dir(sessionPath), "recovery.json")
		state.recoveryWriter = recovery.NewWriter(state.recoveryPath, state.invalidator.Invalidate)
	}
	var stored session.State
	var sessionErr error
	if sessionPath != "" {
		stored, sessionErr = session.Load(sessionPath)
	}
	state.documentMapVisible = stored.DocumentMapVisible
	if len(paths) != 0 {
		state.openArguments(paths, stored.Encodings)
		state.restoreSessionLanguages(stored.Languages)
		state.restoreSessionFolds(stored.Folds)
		state.restoreSessionReadOnly(stored.ReadOnly)
		state.restoreSessionPinned(stored.Pinned)
		state.restoreSessionDirection(stored.RTL)
		state.restoreTabColors(stored.TabColors)
		state.restoreFolderWorkspace(stored)
		state.restoreFunctionDock(stored)
		state.paneDocs[0] = state.space.Current()
		state.resetPaneOwnership()
		state.restoreRecovery(state.space.Current())
		return state
	}
	if sessionPath == "" {
		return state
	}
	if sessionErr != nil {
		state.notice = "Session not restored: " + sessionErr.Error()
	} else {
		for _, path := range stored.Paths {
			if err := state.openRecordedSessionPath(path, stored.Encodings[path]); err != nil {
				state.notice = "Session file unavailable: " + filepath.Base(path)
			}
		}
		state.restoreSessionLanguages(stored.Languages)
		state.restoreSessionFolds(stored.Folds)
		for index, doc := range state.space.Documents {
			if doc.Path == stored.Active {
				state.space.Select(index)
				break
			}
		}
	}
	state.paneDocs[0] = state.space.Current()
	state.resetPaneOwnership()
	state.restoreSessionReadOnly(stored.ReadOnly)
	state.restoreSessionPinned(stored.Pinned)
	state.restoreSessionDirection(stored.RTL)
	state.restoreTabColors(stored.TabColors)
	if sessionErr == nil {
		state.restoreFolderWorkspace(stored)
		state.restoreFunctionDock(stored)
		state.applySessionView(stored.View)
	}
	state.restoreRecovery(nil)
	if state.splitEnabled {
		// The restored active view should accept input on the first frame.
		state.focusViewRequested, state.focusViewArmed = true, true
	}
	return state
}

func (state *model) openRecordedSessionPath(path, label string) error {
	if label == "" {
		return state.space.OpenRecorded(path, document.UTF8)
	}
	format, ok := document.EncodingFromLabel(label)
	if !ok || !format.IsLegacy() {
		return fmt.Errorf("unsupported session encoding %q", label)
	}
	return state.space.OpenRecorded(path, format)
}

func (state *model) openRemembered(path string, encodings map[string]string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	if label := encodings[absolute]; label != "" {
		format, ok := document.EncodingFromLabel(label)
		if !ok {
			return fmt.Errorf("unsupported session encoding %q", label)
		}
		return state.space.OpenWithEncoding(absolute, format)
	}
	return state.space.Open(absolute)
}

func defaultSessionPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "VivaPad", "session.json"), nil
}

func (state *model) applyPreferences(stored preferences.State) {
	state.fontSize, state.subFontSize, state.zoomSync, state.tabWidth = stored.FontSize, stored.SubFontSize, stored.ZoomSync, stored.TabWidth
	state.wrap, state.showLineNumbers = stored.Wrap, stored.LineNumbers
	state.indentMode, state.autoIndent, state.autoClose = stored.IndentMode, stored.AutoIndent, stored.AutoClose
	state.dateTimeFormat, state.dateTimeReverse = stored.DateTimeFormat, stored.DateTimeReverseOrder
	state.showWhitespace, state.showEOL, state.showIndentGuides, state.showWrapSymbol = stored.ShowWhitespace, stored.ShowEOL, stored.ShowIndentGuides, stored.ShowWrapSymbol
	state.showNonPrinting, state.showControls, state.representationsReady = stored.ShowNonPrinting, stored.ShowControls, false
	state.npcMode, state.npcIncludeControls = stored.NPCMode, stored.NPCIncludeControls
	state.printSettings = stored.Print
	state.webSearch = stored.WebSearch
	if stored.ChangeHistoryView == "" {
		stored.ChangeHistoryView = "margin"
	}
	state.changeHistoryView = stored.ChangeHistoryView
}

func (state *model) savePreferences() error {
	if state.preferencesPath == "" {
		state.notice = "Settings not saved: storage unavailable"
		return errors.New("settings storage unavailable")
	}
	stored := preferences.State{
		FontSize: state.fontSize, SubFontSize: state.subFontSize, ZoomSync: state.zoomSync, TabWidth: state.tabWidth,
		Wrap: state.wrap, LineNumbers: state.showLineNumbers,
		IndentMode: state.indentMode, AutoIndent: state.autoIndent, AutoClose: state.autoClose,
		DateTimeFormat: state.dateTimeFormat, DateTimeReverseOrder: state.dateTimeReverse,
		ShowWhitespace: state.showWhitespace, ShowEOL: state.showEOL, ShowIndentGuides: state.showIndentGuides, ShowWrapSymbol: state.showWrapSymbol,
		ShowNonPrinting: state.showNonPrinting, ShowControls: state.showControls,
		NPCMode: state.npcMode, NPCIncludeControls: state.npcIncludeControls,
		ChangeHistoryView: state.changeHistoryView,
		Print:             state.printSettings,
		WebSearch:         state.webSearch,
	}
	if err := preferences.Save(state.preferencesPath, stored); err != nil {
		state.notice = "Settings not saved: " + err.Error()
		return err
	}
	return nil
}

func primary() gfx.Modifiers {
	if runtime.GOOS == "darwin" {
		return gfx.ModifierMeta
	}
	return gfx.ModifierControl
}

func newConfig(state *model) app.Config {
	config := app.Config{
		Tree: &tree, Title: "VivaPad", Width: 1280, Height: 800,
		MinWidth: 640, MinHeight: 400,
		Invalidator: &state.invalidator,
		Icon:        assets.WindowIcon(),
	}
	config.Paint = state.paint
	config.OnActivate = func() {
		if state.recoveryWriter != nil {
			if err, _ := state.recoveryWriter.Status(); err != nil {
				state.queueRecovery(true)
			}
		}
		state.invalidator.Invalidate()
	}
	config.OnFullscreen = state.observeFullscreen
	config.TextInput = func(context app.TextInputContext) *app.TextInput {
		input := state.resolveTextInput(context)
		state.reconcileTextOwner(context.ID, input)
		if input == nil || input.OnEvent == nil || context.ID != editorID && context.ID != rightEditorID {
			return input
		}
		editor, generation := state.compositionOwner.editor, state.compositionGeneration
		onEvent := input.OnEvent
		input.OnEvent = func(event app.TextEvent) (app.TextState, error) {
			if event.CompositionPhase != text.CompositionNone &&
				(state.compositionOwner.target != context.ID || state.compositionOwner.editor != editor || state.compositionGeneration != generation) {
				return app.TextStateOf(editor), nil
			}
			return onEvent(event)
		}
		return input
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		if event.Kind == gfx.PointerDown {
			state.pointerCompositionAway(event.Target)
		}
		if event.Kind == gfx.PointerDown || event.Kind == gfx.PointerUp {
			state.menuPointerShift = event.Modifiers&gfx.ModifierShift != 0
		}
		if result, handled := state.changeHistoryMenuPointer(event); handled {
			return result
		}
		if state.rectangleDragging && (event.Kind == gfx.PointerMove || event.Kind == gfx.PointerUp) {
			pane := state.rectangleDragPane
			doc := state.paneDocument(pane)
			selection := &state.rectangles[pane]
			if selection.doc == doc {
				if offset, virtual, ok := state.codeEditorForPane(pane).OffsetWithVirtualAt(gfx.Point{X: event.X, Y: event.Y}); ok {
					doc.Editor.MoveTo(doc.Editor.RuneIndex(offset), false)
					selection.caret, selection.caretVirtual = doc.Editor.Caret(), virtual
					selection.copyValid = false
				}
			}
			if event.Kind == gfx.PointerUp {
				state.rectangleDragging = false
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if event.Kind == gfx.PointerDown && (event.Target == editorID || event.Target == rightEditorID) {
			pane := paneForTarget(event.Target)
			state.multiSelections[pane] = multiSelectionState{}
			if event.Modifiers&gfx.ModifierAlt != 0 && event.Button == gfx.PointerButtonPrimary {
				state.activatePane(pane)
				doc := state.paneDocument(pane)
				if offset, virtual, ok := state.codeEditorForPane(pane).OffsetWithVirtualAt(gfx.Point{X: event.X, Y: event.Y}); ok {
					doc.Editor.MoveTo(doc.Editor.RuneIndex(offset), false)
					state.rectangles[pane] = rectangleSelection{doc: doc, anchor: doc.Editor.Caret(), caret: doc.Editor.Caret(), anchorVirtual: virtual, caretVirtual: virtual, value: doc.Editor.String(), active: true}
					state.selectAnchors[pane] = selectAnchor{}
					state.rectangleDragging, state.rectangleDragPane = true, pane
					return gfx.EventResult{Handled: true, Repaint: true, Focus: event.Target}
				}
			}
			state.rectangles[pane] = rectangleSelection{}
		}
		if event.Target.Scope() == wordCompletionScope {
			return gfx.EventResult{}
		}
		if event.Kind == gfx.PointerDown {
			state.wordCompletion = nil
		}
		if state.documentMapDragging {
			if event.Kind == gfx.PointerMove || event.Kind == gfx.PointerUp {
				state.scrollFromDocumentMap(event.Y)
				state.syncScrollFrom(state.activePane)
				if event.Kind == gfx.PointerUp {
					state.documentMapDragging = false
				}
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		if state.documentMapVisible && event.Target == documentMapID && event.Kind == gfx.PointerDown {
			state.documentMapDragging = true
			state.scrollFromDocumentMap(event.Y)
			state.syncScrollFrom(state.activePane)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if event.Kind == gfx.PointerDown {
			switch {
			case event.Target == resultsPaneID || event.Target.Scope() == resultScope:
				state.resultFocusMode = true
			case event.Target.Scope() == menuScope || event.Target.Scope() == menuItemScope:
			default:
				state.resultFocusMode = false
			}
		}
		if state.readOnlyDragDoc != nil && (event.Kind == gfx.PointerMove || event.Kind == gfx.PointerUp) {
			doc := state.readOnlyDragDoc
			if event.Kind == gfx.PointerUp || state.documentIndex(doc) < 0 {
				state.readOnlyDragDoc = nil
				return gfx.EventResult{Handled: true}
			}
			if offset, ok := state.codeEditorForPane(state.readOnlyDragPane).OffsetAt(gfx.Point{X: event.X, Y: event.Y}); ok {
				doc.Editor.MoveTo(doc.Editor.RuneIndex(offset), true)
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if event.Kind == gfx.PointerDown && (event.Target == editorID || event.Target == rightEditorID) {
			pane := paneForTarget(event.Target)
			doc := state.paneDocument(pane)
			if state.isReadOnly(doc) {
				state.activatePane(pane)
				if offset, ok := state.codeEditorForPane(pane).OffsetAt(gfx.Point{X: event.X, Y: event.Y}); ok {
					doc.Editor.MoveTo(doc.Editor.RuneIndex(offset), event.Modifiers&gfx.ModifierShift != 0)
					state.readOnlyDragDoc, state.readOnlyDragPane = doc, pane
					return gfx.EventResult{Handled: true, Repaint: true, Focus: event.Target}
				}
			}
		}
		if state.splitEnabled {
			if event.Kind == gfx.PointerDown && event.Target == splitterID {
				state.splitDragging = true
				return gfx.EventResult{Handled: true}
			}
			if state.splitDragging {
				if event.Kind == gfx.PointerMove {
					width := state.splitWidth
					minimum := min(200, max(70, (width-8)/2))
					position := event.X - state.folderPanelWidth(event.Viewport.Width)
					state.splitRatio = min(max(position, minimum), width-minimum-8) / width
					return gfx.EventResult{Handled: true, Repaint: true}
				}
				state.splitDragging = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if pane := paneForTarget(event.Target); pane >= 0 {
				if event.Kind == gfx.PointerDown {
					state.activatePane(pane)
				}
				result := state.codeEditorForPane(pane).Pointer(event)
				state.syncScrollFrom(pane)
				return result
			}
		}
		return state.codeEditor().Pointer(event)
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult {
		if state.saveSessionOptionsOpen {
			return gfx.EventResult{Handled: true}
		}
		if state.byteRangeOpen && (event.Target == byteRangePanelID || event.Target.Scope() == byteRangeModeScope || event.Target == byteRangeStartID || event.Target == byteRangeEndID) {
			return gfx.EventResult{Handled: true}
		}
		if state.columnDialogOpen {
			return gfx.EventResult{Handled: true}
		}
		if state.preferencesOpen {
			if state.printPreferenceField(event.Target) {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.menuOpen == 0 && (event.Target == fileMenuPanelID || event.Target == fileMenuPreviousID || event.Target == fileMenuNextID || event.Target.Scope() == menuItemScope) {
			step := 1
			if event.StepY(20) < 0 {
				step = -1
			}
			state.fileMenuOffset = min(max(0, state.fileMenuOffset+step), max(0, fileMenuCount()-state.fileMenuVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.menuOpen == 9 && (event.Target == runMenuPanelID || event.Target == runMenuPreviousID || event.Target == runMenuNextID || event.Target.Scope() == runNamedScope) {
			step := 1
			if event.StepY(23) < 0 {
				step = -1
			}
			state.runMenuOffset = min(max(0, state.runMenuOffset+step), max(0, len(state.namedRuns)+2-state.runMenuVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.menuOpen == 1 && (event.Target == editMenuPanelID || event.Target == editMenuPreviousID || event.Target == editMenuNextID || event.Target.Scope() == menuItemScope) {
			step := 1
			if event.StepY(24) < 0 {
				step = -1
			}
			state.editMenuOffset = min(max(0, state.editMenuOffset+step), max(0, len(menuCommands[1])-state.editMenuVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.menuOpen == 2 && (event.Target == searchMenuPanelID || event.Target == searchMenuPreviousID || event.Target == searchMenuNextID || event.Target.Scope() == menuItemScope) {
			step := 1
			if event.StepY(24) < 0 {
				step = -1
			}
			state.searchMenuOffset = min(max(0, state.searchMenuOffset+step), max(0, len(menuCommands[2])-state.searchMenuVisible))
			state.closeSearchSubmenus()
			return gfx.EventResult{Handled: true, Repaint: true, Focus: searchMenuPanelID}
		}
		if state.markJumpMenuOpen && event.Target.Scope() == markJumpScope {
			return gfx.EventResult{Handled: true}
		}
		if state.styleMenuMode != "" && event.Target.Scope() == styleMenuScope {
			return gfx.EventResult{Handled: true}
		}
		if state.menuOpen == 3 && (event.Target == viewMenuPanelID || event.Target == viewMenuPreviousID || event.Target == viewMenuNextID || event.Target.Scope() == menuItemScope) {
			step := 1
			if event.StepY(20) < 0 {
				step = -1
			}
			state.viewMenuOffset = min(max(0, state.viewMenuOffset+step), max(0, len(menuCommands[3])-state.viewMenuVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.summaryOpen || state.helpMode != noHelp || state.webSearchOpen {
			return gfx.EventResult{Handled: true}
		}
		if state.activeDock() == "characters" && (event.Target == characterPanelID || event.Target.Scope() == characterCellScope) {
			step := 1
			if event.StepY(21) < 0 {
				step = -1
			}
			state.moveCharacterPanel(step)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.documentListVisible && state.activeDock() == "documents" &&
			(event.Target == documentListPanelID || event.Target.Scope() == documentListRowScope || event.Target.Scope() == documentListRowCloseScope) {
			step := 1
			if event.StepY(23) < 0 {
				step = -1
			}
			state.documentListOffset = min(max(0, state.documentListOffset+step), max(0, len(state.documentListRows)-state.documentListVisibleRows))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.menuOpen == 11 && (event.Target == windowRecentPanelID || event.Target == windowRecentPreviousID || event.Target == windowRecentNextID || event.Target.Scope() == windowRecentScope) {
			step := 1
			if event.StepY(23) < 0 {
				step = -1
			}
			state.windowRecentOffset = min(max(0, state.windowRecentOffset+step), max(0, min(40, len(state.activeViewTabs()))-state.windowRecentVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.windowsDialogOpen {
			if event.Target == windowsPanelID || event.Target.Scope() == windowsRowScope {
				return state.scrollWindowsDialog(event)
			}
			return gfx.EventResult{Handled: true}
		}
		if state.functionListVisible && state.activeDock() == "functions" &&
			(event.Target == functionPanelID || event.Target.Scope() == functionSymbolScope) {
			return state.scrollFunctionList(event)
		}
		if state.documentMapVisible && event.Target == documentMapID {
			result := state.codeEditor().Scroll(event)
			state.syncScrollFrom(state.activePane)
			return result
		}
		if state.menuOpen == 5 &&
			(event.Target == languagePanelID || event.Target == languagePrevID || event.Target == languageNextID || event.Target.Scope() == menuItemScope) {
			step := 1
			if event.StepY(22) < 0 {
				step = -1
			}
			state.languageOffset = min(max(0, state.languageOffset+step), max(0, len(syntax.Languages())-state.languageVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.macroRunOpen && state.macroSourceOpen &&
			(event.Target == macroSourcePrevID || event.Target == macroSourceNextID || event.Target.Scope() == macroSourceScope) {
			step := 1
			if event.StepY(24) < 0 {
				step = -1
			}
			state.macroSourceOffset = min(max(0, state.macroSourceOffset+step), max(0, state.macroSourceCount()-state.macroSourceVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.renameDoc != nil || state.renamePlan != nil || state.pendingTrash != nil {
			return gfx.EventResult{Handled: true}
		}
		if state.hashDialogOpen {
			if event.Target == hashInputID && state.hashMode == hashFromText {
				state.hashInputLayout.ScrollBy(event.DeltaY)
			} else if event.Target == hashResultID {
				state.hashResultLayout.ScrollBy(event.DeltaY)
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.lineMenuOpen && state.menuOpen == 1 &&
			(event.Target == linePanelID || event.Target == linePrevID || event.Target == lineNextID || event.Target.Scope() == lineScope) {
			step := 1
			if event.StepY(24) < 0 {
				step = -1
			}
			state.lineMenuOffset = min(max(0, state.lineMenuOffset+step), max(0, len(lineCommands)-state.lineMenuVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.charsetMenuOpen && (state.menuOpen == 0 || state.menuOpen == 4) &&
			(event.Target == charsetPanelID || event.Target == charsetPrevID || event.Target == charsetNextID || event.Target.Scope() == charsetScope) {
			step := 1
			if event.StepY(22) < 0 {
				step = -1
			}
			state.charsetOffset = min(max(0, state.charsetOffset+step), max(0, len(document.LegacyEncodings())-state.charsetVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.savedMacroMenuOpen && state.menuOpen == 8 &&
			(event.Target == savedMacroPanelID || event.Target == savedMacroPrevID || event.Target == savedMacroNextID || event.Target.Scope() == savedMacroScope) {
			step := 1
			if event.StepY(22) < 0 {
				step = -1
			}
			state.savedMacroOffset = min(max(0, state.savedMacroOffset+step), max(0, len(state.namedMacros)-state.savedMacroVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.viewTabMenuOpen && state.menuOpen == 3 &&
			(event.Target == viewTabPanelID || event.Target == viewTabPreviousID || event.Target == viewTabNextID || event.Target.Scope() == viewTabScope) {
			step := 1
			if event.StepY(21) < 0 {
				step = -1
			}
			state.viewTabOffset = min(max(0, state.viewTabOffset+step), max(0, len(viewTabCommands)-state.viewTabVisible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.folderVisible && (event.Target == folderPanelID || event.Target.Scope() == folderRowScope) {
			step := 1
			if event.StepY(22) < 0 {
				step = -1
			}
			visible := max(1, int((event.Viewport.Height-menuHeight-toolbarHeight-tabHeight-statusHeight-36)/22))
			state.folderOffset = min(max(0, state.folderOffset+step), max(0, len(state.folderRows())-visible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if event.Target == resultsPaneID || event.Target.Scope() == resultScope {
			step := 1
			if event.StepY(22) < 0 {
				step = -1
			}
			visible := max(1, int((resultPaneHeight(event.Viewport.Height)-30)/22))
			state.resultOffset = min(max(0, state.resultOffset+step), max(0, len(state.fileResults.Matches)-visible))
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		pane := paneForTarget(event.Target)
		if pane < 0 || pane == 1 && !state.splitEnabled {
			return gfx.EventResult{}
		}
		result := state.codeEditorForPane(pane).Scroll(event)
		state.syncScrollFrom(pane)
		return result
	}
	config.OnFileDrop = func(event gfx.FileDropEvent) gfx.EventResult {
		if state.saveSessionOptionsOpen || state.columnDialogOpen || state.preferencesOpen || state.runDialogOpen || state.summaryOpen || state.helpMode != noHelp || state.webSearchOpen || state.pendingClose >= 0 || state.pendingReload >= 0 || state.filesDialogOpen || state.replaceConfirmOpen || state.renameDoc != nil || state.renamePlan != nil || state.pendingTrash != nil || state.dateTimeDialogOpen || state.hashDialogOpen || state.windowsDialogOpen {
			return gfx.EventResult{}
		}
		if event.Phase == gfx.FileDropDrop {
			if pane := paneForTarget(event.Target); pane >= 0 {
				state.activatePane(pane)
			}
			if state.openDocumentPaths(event.Paths) {
				if state.recentOpen {
					// Opening files can replace the row under a held menu click.
					state.menuOpen, state.recentOpen = -1, false
				}
				return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}
			}
		}
		return gfx.EventResult{Handled: true, Repaint: event.Phase == gfx.FileDropDrop}
	}
	config.OnShortcut = func(event gfx.KeyEvent) gfx.EventResult {
		if state.saveSessionOptionsOpen {
			return gfx.EventResult{Handled: true}
		}
		if state.byteRangeOpen && event.Target == byteRangePanelID {
			return gfx.EventResult{Handled: true}
		}
		if state.columnDialogOpen {
			if state.columnField(event.Target) {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.preferencesOpen {
			return gfx.EventResult{Handled: true}
		}
		if state.runDialogOpen {
			if event.Target == runDialogInputID || event.Target == runDialogNameID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.summaryOpen || state.helpMode != noHelp {
			return gfx.EventResult{Handled: true}
		}
		if state.webSearchOpen {
			return gfx.EventResult{Handled: event.Target != webSearchTemplateID}
		}
		if state.windowsDialogOpen {
			if event.Key == gfx.KeyUnknown && event.Text == " " {
				return state.windowsDialogSpace(event)
			}
			return gfx.EventResult{Handled: true}
		}
		if state.functionListVisible && state.activeDock() == "functions" && event.Target == functionFilterID {
			return gfx.EventResult{}
		}
		if state.macroSaveOpen || state.macroRunOpen {
			if event.Target == macroNameID || event.Target == macroCountID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.hashDialogOpen {
			if state.hashMode == hashFromText && event.Target == hashInputID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.dateTimeDialogOpen {
			if event.Target == dateTimeFieldID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.renameDoc != nil {
			if event.Target == renameFieldID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.renamePlan != nil || state.pendingTrash != nil {
			return gfx.EventResult{Handled: true}
		}
		if state.filesDialogOpen {
			if state.filesDialogField(event.Target) {
				plainText := event.Key == gfx.KeyUnknown && event.Text != "" && event.Modifiers&^gfx.ModifierShift == 0
				// GTK offers the input-method toggle here before filtering it through the IME.
				inputMethodToggle := event.Modifiers == gfx.ModifierControl &&
					(event.Key == gfx.KeySpace || event.Key == gfx.KeyUnknown && event.Text == " ")
				if plainText || inputMethodToggle {
					return gfx.EventResult{}
				}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.pendingClose >= 0 || state.pendingReload >= 0 || state.replaceConfirmOpen {
			return gfx.EventResult{Handled: true}
		}
		if command := directionShortcut(event); command != "" {
			state.activatePane(paneForTarget(event.Target))
			state.command(command)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if pane := paneForTarget(event.Target); pane >= 0 && (event.Target == editorID || event.Target == rightEditorID) &&
			state.isReadOnly(state.paneDocument(pane)) && event.Modifiers == primary() {
			switch strings.ToLower(event.Text) {
			case "c":
				state.activatePane(pane)
				state.command("Copy")
				return gfx.EventResult{Handled: true, Repaint: true}
			case "x":
				state.notice = "Read-only document"
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		if event.Modifiers == gfx.ModifierAlt|gfx.ModifierShift && strings.EqualFold(event.Text, "b") {
			state.command("Begin/End Select in Column Mode")
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if event.Modifiers&primary() == 0 {
			return gfx.EventResult{}
		}
		if strings.ToLower(event.Text) == "i" && event.Modifiers == primary()|gfx.ModifierAlt {
			state.openIncrementalSearch(state.incrementOpen && event.Target == incrementQueryID)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if (event.Key == gfx.KeySpace || event.Text == " ") && event.Modifiers == primary()|gfx.ModifierShift {
			if pane := paneForTarget(event.Target); pane >= 0 && (event.Target == editorID || event.Target == rightEditorID) {
				state.activatePane(pane)
				state.command("Path Completion")
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		if event.Key == gfx.KeyEnter && event.Modifiers == primary() {
			if pane := paneForTarget(event.Target); pane >= 0 && (event.Target == editorID || event.Target == rightEditorID) {
				state.activatePane(pane)
				state.command("Word Completion")
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		if event.Modifiers&gfx.ModifierShift != 0 {
			switch strings.ToLower(event.Text) {
			case "r":
				if state.macroRecording {
					state.command("Stop Recording")
				} else {
					state.command("Record Macro")
				}
				return gfx.EventResult{Handled: true, Repaint: true}
			case "p":
				state.command("Playback Macro")
				return gfx.EventResult{Handled: true, Repaint: true}
			case "t":
				state.restoreClosed()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		if pane := paneForTarget(event.Target); pane >= 0 && (event.Target == editorID || event.Target == rightEditorID) {
			state.activatePane(pane)
			if command, ok := components.CodeEditorCommandForKey(event); ok {
				if command == components.CodeEditorToggleComment {
					state.command("Toggle Single Line Comment")
					return gfx.EventResult{Handled: true, Repaint: true}
				}
				if state.isReadOnly(state.paneDocument(pane)) {
					state.notice = "Read-only document"
					return gfx.EventResult{Handled: true, Repaint: true}
				}
				state.applyCodeCommand(state.paneDocument(pane), command)
				state.recordMacro(state.paneDocument(pane), macro.Step{Kind: macro.CodeStep, Code: command})
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		switch strings.ToLower(event.Text) {
		case "b":
			if event.Modifiers&gfx.ModifierShift != 0 {
				state.command("Begin/End Select")
			} else if event.Modifiers&gfx.ModifierAlt != 0 {
				state.command("Select All In-between {} [] or ()")
			} else {
				state.command("Go to Matching Brace")
			}
		case "n":
			state.cancelMacroForCommand()
			state.showInActivePane(state.space.NewDocument())
		case "o":
			state.cancelMacroForCommand()
			if state.menuOpen >= 0 {
				state.command("Open...")
			} else {
				state.openFiles()
			}
		case "s":
			state.cancelMacroForCommand()
			if state.menuOpen >= 0 {
				if event.Modifiers&gfx.ModifierShift != 0 {
					state.command("Save As...")
				} else {
					state.command("Save")
				}
			} else {
				state.save(state.space.Active, event.Modifiers&gfx.ModifierShift != 0)
			}
		case "f", "h":
			state.cancelMacroForCommand()
			state.closeIncrementalSearch()
			state.markDialogOpen = false
			if event.Text == "f" && event.Modifiers&gfx.ModifierShift != 0 {
				state.openFilesDialog()
				return gfx.EventResult{Handled: true, Repaint: true, Focus: queryID}
			}
			state.searchOpen, state.goToOpen = true, false
			return gfx.EventResult{Handled: true, Repaint: true, Focus: queryID}
		case "g":
			state.cancelMacroForCommand()
			state.closeIncrementalSearch()
			state.goToOpen, state.searchOpen = true, false
			return gfx.EventResult{Handled: true, Repaint: true, Focus: goToFieldID}
		case "d":
			state.command("Duplicate Line")
		case "l":
			state.command("Delete Line")
		case "u":
			if state.isReadOnly(state.space.Current()) {
				state.notice = "Read-only document"
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			state.cancelMacroForCommand()
			if event.Modifiers&gfx.ModifierShift != 0 {
				state.convertSelectedCase(state.space.Current(), document.CaseUpper, "Converted to uppercase")
			} else {
				state.convertSelectedCase(state.space.Current(), document.CaseLower, "Converted to lowercase")
			}
		case "+", "=":
			state.cancelMacroForCommand()
			state.zoom("Zoom In")
		case "-":
			state.cancelMacroForCommand()
			state.zoom("Zoom Out")
		case "0":
			state.cancelMacroForCommand()
			state.zoom("Restore Default Zoom")
		case "w":
			state.cancelMacroForCommand()
			state.closePaneTab(state.activePane, state.paneDocument(state.activePane))
		default:
			return gfx.EventResult{}
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		if result := state.postItShortcut(event); result.Handled {
			return result
		}
		if state.markDialogOpen {
			if event.Key == gfx.KeyEscape && state.menuOpen < 0 {
				state.markDialogOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if event.Key == gfx.KeyEnter && event.Target == queryID {
				state.markAll()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		if state.saveSessionOptionsOpen {
			if event.Key == gfx.KeyEscape {
				state.saveSessionOptionsOpen = false
			} else if event.Key == gfx.KeyEnter {
				state.saveNPPSession(state.saveSessionIncludeWorkspace)
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.byteRangeOpen {
			if event.Key == gfx.KeyEscape {
				state.byteRangeOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if event.Key == gfx.KeyEnter && (event.Target == byteRangeStartID || event.Target == byteRangeEndID || event.Target == byteRangeFindID) {
				state.findByteRange()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if event.Target == byteRangeStartID || event.Target == byteRangeEndID {
				return gfx.EventResult{}
			}
		}
		if state.columnDialogOpen {
			if event.Key == gfx.KeyEscape {
				state.columnDialogOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if event.Key == gfx.KeyEnter && state.columnField(event.Target) {
				state.applyColumnDialog()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.columnField(event.Target) {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.preferencesOpen {
			if event.Key == gfx.KeyEscape {
				state.closePreferences()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.printPreferenceField(event.Target) {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.runDialogOpen {
			switch event.Key {
			case gfx.KeyEscape:
				state.cancelRunDialog()
				return gfx.EventResult{Handled: true, Repaint: true}
			case gfx.KeyEnter:
				if event.Target == runDialogInputID || event.Target == runDialogNameID {
					state.applyRunDialog()
					return gfx.EventResult{Handled: true, Repaint: true}
				}
			}
			if event.Target == runDialogInputID || event.Target == runDialogNameID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.helpMode != noHelp {
			if event.Key == gfx.KeyEscape {
				state.closeHelp()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			return gfx.EventResult{}
		}
		if state.webSearchOpen {
			switch {
			case event.Key == gfx.KeyEscape:
				state.closeWebSearchSettings()
			case event.Key == gfx.KeyEnter && event.Target == webSearchTemplateID:
				state.saveWebSearchSettings()
			default:
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.summaryOpen {
			if event.Key == gfx.KeyEscape || event.Key == gfx.KeyEnter {
				state.summaryOpen = false
				return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.windowsDialogOpen {
			return state.windowsDialogKey(event)
		}
		if state.functionListVisible && state.activeDock() == "functions" && event.Target == functionFilterID {
			if result, handled := state.functionListKey(event); handled {
				return result
			}
		}
		if state.wordCompletion != nil {
			if !state.validWordCompletion() {
				state.wordCompletion = nil
			} else {
				switch event.Key {
				case gfx.KeyEscape:
					state.wordCompletion = nil
					focus := editorID
					if state.activePane == 1 {
						focus = rightEditorID
					}
					return gfx.EventResult{Handled: true, Repaint: true, Focus: focus}
				case gfx.KeyDown, gfx.KeyUp:
					current := state.wordCompletion
					step := 1
					if event.Key == gfx.KeyUp {
						step = -1
					}
					current.selected = (current.selected + step + len(current.items)) % len(current.items)
					state.wordFocusPending = true
					return gfx.EventResult{Handled: true, Repaint: true}
				case gfx.KeyEnter, gfx.KeyTab:
					state.acceptWordCompletion(state.wordCompletion.selected)
					focus := editorID
					if state.activePane == 1 {
						focus = rightEditorID
					}
					return gfx.EventResult{Handled: true, Repaint: true, Focus: focus}
				default:
					state.wordCompletion = nil
				}
			}
		}
		if result, handled := state.changeHistoryMenuKey(event); handled {
			return result
		}
		if result, handled := state.bookmarkMenuKey(event); handled {
			return result
		}
		if result, handled := state.recentMenuKey(event); handled {
			return result
		}
		if event.Key == gfx.KeyEscape && state.menuOpen < 0 && !state.menuOverflowOpen && state.calltip != nil {
			state.calltip = nil
			return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}
		}
		if event.Key == gfx.KeyEscape {
			if state.styleMenuMode != "" {
				state.styleMenuMode = ""
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.markJumpMenuOpen {
				state.markJumpMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.macroSaveOpen || state.macroRunOpen {
				state.closeMacroDialog()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.hashDialogOpen {
				state.closeHashDialog()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.dateTimeDialogOpen {
				state.closeDateTimeDialog()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.renameDoc != nil || state.renamePlan != nil {
				state.cancelRename()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.pendingTrash != nil {
				state.dismissTrash()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.replaceConfirmOpen {
				state.cancelReplacePreview()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.filesDialogOpen {
				state.cancelFilesDialog()
				return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}
			}
			if state.selectedSearchMenuOpen {
				state.selectedSearchMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.resultsOpen && !state.distractionFree && !state.incrementOpen {
				state.discardFileResults()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.pendingClose >= 0 {
				state.cancelClose()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.pendingReload >= 0 {
				state.cancelReload()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.recentOpen {
				state.recentOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.lineMenuOpen {
				state.lineMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.charsetMenuOpen {
				state.charsetMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.savedMacroMenuOpen {
				state.savedMacroMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.insertDateOpen {
				state.insertDateOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.hashMenuOpen {
				state.hashMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.symbolMenuOpen {
				state.symbolMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.viewTabMenuOpen {
				state.viewTabMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.namedBrowserMenuOpen {
				state.namedBrowserMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.foldMenuOpen {
				state.foldMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.windowSortMenuOpen {
				state.windowSortMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.menuOverflowOpen {
				state.menuOverflowOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.caseMenuOpen {
				state.caseMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.commentMenuOpen {
				state.commentMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.eolMenuOpen {
				state.eolMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.sessionMenuOpen {
				state.sessionMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.bookmarkMenuOpen {
				state.bookmarkMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.changeHistoryMenuOpen {
				state.changeHistoryMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.closeMultipleMenuOpen {
				state.closeMultipleMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.containingMenuOpen {
				state.containingMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.autoMenuOpen {
				state.autoMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.blankMenuOpen {
				state.blankMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.copyPathsMenuOpen {
				state.copyPathsMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.pasteSpecialOpen {
				state.pasteSpecialOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.onSelectionMenuOpen {
				state.onSelectionMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.multiAllMenuOpen || state.multiNextMenuOpen {
				state.multiAllMenuOpen, state.multiNextMenuOpen = false, false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.readOnlyMenuOpen {
				state.readOnlyMenuOpen = false
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.menuOpen >= 0 || state.searchOpen || state.goToOpen {
				result := gfx.EventResult{Handled: true, Repaint: true}
				if state.searchOpen || state.goToOpen {
					result.Focus = state.commandEditorFocusID()
				}
				state.menuOpen, state.searchOpen, state.goToOpen = -1, false, false
				return result
			}
			if state.incrementOpen {
				state.closeIncrementalSearch()
				focus := editorID
				if state.splitEnabled && state.activePane == 1 {
					focus = rightEditorID
				}
				return gfx.EventResult{Handled: true, Repaint: true, Focus: focus}
			}
			if state.postIt.active {
				state.command(postItCommand)
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if state.fullscreen {
				state.command(state.fullscreenExitCommand())
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		if state.renameDoc != nil {
			if event.Key == gfx.KeyEnter && event.Target == renameFieldID {
				state.applyUntitledRename()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			return gfx.EventResult{}
		}
		if state.dateTimeDialogOpen {
			if event.Key == gfx.KeyEnter && event.Target == dateTimeFieldID {
				state.applyDateTimeSettings()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if event.Target == dateTimeFieldID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.macroSaveOpen || state.macroRunOpen {
			if event.Key == gfx.KeyEnter && (event.Target == macroNameID || event.Target == macroCountID) {
				state.applyMacroDialog()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if event.Target == macroNameID || event.Target == macroCountID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.hashDialogOpen {
			if state.hashMode == hashFromText && event.Target == hashInputID {
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true}
		}
		if state.renamePlan != nil || state.pendingTrash != nil {
			return gfx.EventResult{Handled: true}
		}
		if state.filesDialogOpen {
			if event.Key == gfx.KeyEnter && state.filesDialogField(event.Target) {
				if state.filesReplaceMode {
					state.startReplacePreview()
				} else {
					state.startFileSearch()
				}
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			return gfx.EventResult{}
		}
		if state.pendingClose >= 0 || state.pendingReload >= 0 || state.replaceConfirmOpen {
			return gfx.EventResult{Handled: true}
		}
		if result := state.folderWorkspaceKey(event); result.Handled {
			return result
		}
		if event.Key == gfx.KeyF7 && event.Modifiers == 0 {
			if state.distractionFree {
				return gfx.EventResult{Handled: true}
			}
			if !state.resultsOpen || len(state.fileResults.Matches) == 0 {
				state.notice = "No search results"
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			state.searchOpen, state.goToOpen = false, false
			state.menuOpen = -1
			if event.Target == resultsPaneID || event.Target.Scope() == resultScope {
				state.resultFocusMode = false
				focus := editorID
				if state.splitEnabled && state.activePane == 1 {
					focus = rightEditorID
				}
				return gfx.EventResult{Handled: true, Repaint: true, Focus: focus}
			}
			state.resultFocusMode = true
			return gfx.EventResult{Handled: true, Repaint: true, Focus: resultsPaneID}
		}
		if state.resultsOpen && (event.Target == resultsPaneID || event.Target.Scope() == resultScope) {
			index := state.resultSelected
			handled := true
			switch event.Key {
			case gfx.KeyDown:
				index++
			case gfx.KeyUp:
				if index < 0 {
					index = len(state.fileResults.Matches)
				}
				index--
			case gfx.KeyHome:
				index = 0
			case gfx.KeyEnd:
				index = len(state.fileResults.Matches) - 1
			case gfx.KeyPageDown:
				index += max(1, state.resultVisible)
			case gfx.KeyPageUp:
				index -= max(1, state.resultVisible)
			case gfx.KeyEnter:
				if index >= 0 && state.openFileResult(index) {
					state.resultFocusMode = false
					focus := editorID
					if state.splitEnabled && state.activePane == 1 {
						focus = rightEditorID
					}
					return gfx.EventResult{Handled: true, Repaint: true, Focus: focus}
				}
				return gfx.EventResult{Handled: true, Repaint: true}
			default:
				handled = false
			}
			if handled {
				if len(state.fileResults.Matches) != 0 {
					state.selectFileResult(min(max(0, index), len(state.fileResults.Matches)-1))
				}
				return gfx.EventResult{Handled: true, Repaint: true, Focus: resultsPaneID}
			}
		}
		if event.Key == gfx.KeyF4 && event.Modifiers&^gfx.ModifierShift == 0 {
			fromPane := state.activePane
			if event.Modifiers&gfx.ModifierShift != 0 {
				state.command("Previous Search Result")
			} else {
				state.command("Next Search Result")
			}
			result := gfx.EventResult{Handled: true, Repaint: true}
			if state.splitEnabled && paneForTarget(event.Target) == fromPane && state.activePane != fromPane {
				result.Focus = editorID
				if state.activePane == 1 {
					result.Focus = rightEditorID
				}
			}
			return result
		}
		if event.Key == gfx.KeyF2 {
			if pane := paneForTarget(event.Target); pane >= 0 {
				state.activatePane(pane)
			}
			if event.Modifiers&gfx.ModifierControl != 0 {
				state.toggleBookmark()
			} else {
				state.navigateBookmark(event.Modifiers&gfx.ModifierShift != 0)
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if event.Key == gfx.KeyTab && event.Modifiers&gfx.ModifierControl != 0 {
			state.cycleTabs(event.Modifiers&gfx.ModifierShift != 0)
			focus := editorID
			if state.splitEnabled && state.activePane == 1 {
				focus = rightEditorID
			}
			return gfx.EventResult{Handled: true, Repaint: true, Focus: focus}
		}
		if event.Key == gfx.KeyF3 {
			if state.incrementOpen && event.Target == incrementQueryID && event.Modifiers&^gfx.ModifierShift == 0 {
				state.findIncremental(event.Modifiers&gfx.ModifierShift != 0, true)
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if pane := paneForTarget(event.Target); pane >= 0 {
				state.activatePane(pane)
			}
			switch event.Modifiers {
			case 0, gfx.ModifierShift:
				state.cancelMacroForCommand()
				state.find(event.Modifiers&gfx.ModifierShift != 0)
			case primary():
				state.command("Select and Find Next")
			case primary() | gfx.ModifierShift:
				state.command("Select and Find Previous")
			case primary() | gfx.ModifierAlt:
				state.command("Find (Volatile) Next")
			case primary() | gfx.ModifierAlt | gfx.ModifierShift:
				state.command("Find (Volatile) Previous")
			default:
				return gfx.EventResult{}
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.searchOpen && event.Target == queryID && event.Key == gfx.KeyEnter {
			state.find(false)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if event.Target == goToFieldID && event.Key == gfx.KeyEnter {
			state.goToLine()
			return gfx.EventResult{Handled: true, Repaint: true, Focus: editorID}
		}
		if state.incrementOpen && event.Target == incrementQueryID && event.Key == gfx.KeyEnter {
			state.findIncremental(event.Modifiers&gfx.ModifierShift != 0, true)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if result := state.splitKey(event, state.splitWidth); result.Handled {
			return result
		}
		pane := paneForTarget(event.Target)
		if pane < 0 || event.Target != editorID && event.Target != rightEditorID {
			return gfx.EventResult{}
		}
		state.activatePane(pane)
		doc := state.paneDocument(pane)
		if state.extendRectangleKey(pane, event) {
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.moveShapedHorizontal(pane, doc, event) {
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if state.isReadOnly(doc) && (event.Key == gfx.KeyLeft || event.Key == gfx.KeyRight) {
			modifiers := event.Modifiers &^ gfx.ModifierShift
			extend := event.Modifiers&gfx.ModifierShift != 0
			switch {
			case modifiers == 0:
				if event.Key == gfx.KeyLeft {
					doc.Editor.MoveLeft(extend)
				} else {
					doc.Editor.MoveRight(extend)
				}
				return gfx.EventResult{Handled: true, Repaint: true}
			case modifiers == gfx.ModifierControl || runtime.GOOS == "darwin" && modifiers == gfx.ModifierAlt:
				doc.Editor.MoveWord(event.Key == gfx.KeyRight, extend)
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		if command, ok := editorCodeCommandForKey(event); ok {
			if state.isReadOnly(doc) {
				state.notice = "Read-only document"
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			state.applyCodeCommand(doc, command)
			state.recordMacro(doc, macro.Step{Kind: macro.CodeStep, Code: command})
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
		if movement == components.TextareaMoveNone {
			return gfx.EventResult{}
		}
		value := doc.Editor.String()
		next := state.codeEditorForPane(pane).Move(value, doc.Editor.ByteOffset(doc.Editor.Caret()), movement)
		doc.Editor.MoveTo(doc.Editor.RuneIndex(next), extend)
		if state.macroRecording {
			state.cancelMacroUnsupported("movement")
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	config.OnCloseRequest = func(allow func()) {
		// The pending exit already owns the session snapshot and close callback.
		if state.allowClose != nil {
			return
		}
		if state.saveSessionOptionsOpen {
			state.notice = "Finish or cancel Save Session before closing"
			state.invalidator.Invalidate()
			return
		}
		if state.columnDialogOpen {
			state.notice = "Finish or cancel Column Editor before closing"
			state.invalidator.Invalidate()
			return
		}
		if state.preferencesOpen {
			state.notice = "Close Preferences before exiting"
			state.invalidator.Invalidate()
			return
		}
		if state.runDialogOpen {
			state.notice = "Finish or cancel Run before closing"
			state.invalidator.Invalidate()
			return
		}
		if state.renameDoc != nil || state.renamePlan != nil {
			state.notice = "Finish or cancel Rename before closing"
			state.invalidator.Invalidate()
			return
		}
		if state.pendingTrash != nil {
			state.notice = "Finish or cancel Move to Recycle Bin before closing"
			state.invalidator.Invalidate()
			return
		}
		if state.batchActive {
			state.notice = "Finish or cancel the batch close before exiting"
			state.invalidator.Invalidate()
			return
		}
		if state.pendingReload >= 0 {
			state.notice = "Finish or cancel Reload from Disk before closing"
			state.invalidator.Invalidate()
			return
		}
		if state.replaceConfirmOpen {
			state.notice = "Finish or cancel Replace in Files before closing"
			state.invalidator.Invalidate()
			return
		}
		state.closingDocs = append(state.closingDocs[:0], state.space.Documents...)
		state.closingTabSettings = nil
		state.closingActive = state.space.Current()
		state.closingPaneDocs, state.closingSplit = state.paneDocs, state.splitEnabled
		state.closingActivePane, state.closingDivider = state.activePane, state.splitRatio
		state.closingOwners = make(map[*document.Document]int, len(state.paneOwner))
		for doc, pane := range state.paneOwner {
			state.closingOwners[doc] = pane
		}
		for index, doc := range state.space.Documents {
			if doc.Dirty() {
				state.pendingClose, state.allowClose = index, allow
				state.invalidator.Invalidate()
				return
			}
		}
		state.persistSession()
		allow()
	}
	config.OnWake = func() {
		state.consumeFileSearch()
		state.consumeFolderChanges()
		state.consumeMonitorChanges()
		state.observeRecoveryError()
	}
	config.OnClosed = func() {
		state.cancelFileSearch()
		state.stopFolderWatching()
		state.stopAllMonitors()
		if state.recoveryWriter != nil {
			state.recoveryWriter.Wait()
		}
	}
	return config
}

func (state *model) codeEditor() *components.CodeEditor {
	return state.codeEditorForPane(state.activePane)
}

func (state *model) languageFor(doc *document.Document) string {
	if language, selected := state.languages[doc]; selected {
		return language
	}
	return strings.ToLower(filepath.Ext(doc.Name))
}

func (state *model) configureCode(doc *document.Document) {
	options, known := codeOptions(state.languageFor(doc))
	if !known {
		options = text.CodeOptions{TabWidth: 4, HardTabs: true}
	} else {
		options.AutoIndent, options.AutoClose = state.autoIndent, state.autoClose
	}
	options.TabWidth = state.tabWidth
	switch state.indentMode {
	case "tabs":
		options.HardTabs = true
	case "spaces":
		options.HardTabs = false
	}
	doc.Editor.SetCode(options)
}

func codeOptions(language string) (text.CodeOptions, bool) {
	options := text.CodeOptions{TabWidth: 4, AutoIndent: true, AutoClose: true}
	if spec, ok := syntax.LanguageByMode(language); ok && spec.Lexer != "" {
		options.IndentAfter, options.LineComment = spec.IndentAfter, spec.LineComment
		return options, true
	}
	switch strings.ToLower(language) {
	case ".go", ".c", ".cpp", ".h", ".js", ".ts", ".java", ".rs":
		options.IndentAfter, options.LineComment = "{[(", "//"
	case ".py", ".sh", ".yaml", ".yml", ".toml":
		options.IndentAfter, options.LineComment = ":", "#"
	case ".json":
		options.IndentAfter = "{[("
	default:
		return text.CodeOptions{}, false
	}
	return options, true
}

func (state *model) openFiles() {
	state.cancelFileCommandComposition(state.space.Current())
	defer state.requestNativeChooserFocus()
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: "Open", AllowMultiple: true})
	if err != nil {
		if !errors.Is(err, app.ErrCanceled) {
			state.notice = err.Error()
		}
		return
	}
	state.openDocumentPaths(paths)
}

func (state *model) openDocumentPaths(paths []string) bool {
	return state.openDocumentBatch(paths, state.space.OpenInteractive)
}

func (state *model) openDocumentBatch(paths []string, open func(string) error) (opened bool) {
	for _, path := range paths {
		if err := state.openIntoWorkspace(func() error { return open(path) }); err != nil {
			state.notice = err.Error()
		} else {
			doc := state.space.Current()
			// Keep new tabs in the initiating pane until the batch is complete.
			if _, owned := state.paneOwner[doc]; !owned {
				state.showInActivePane(doc)
			}
			state.rememberRecent(doc)
			opened = true
		}
	}
	if opened {
		state.activateOpenedDocument(state.space.Current())
	}
	return opened
}

func (state *model) openFilesWithEncoding(encoding document.Encoding) {
	state.cancelFileCommandComposition(state.space.Current())
	defer state.requestNativeChooserFocus()
	spec, _ := encoding.LegacySpec()
	title := "Open as " + spec.Label
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: title, AllowMultiple: true})
	if err != nil {
		if !errors.Is(err, app.ErrCanceled) {
			state.notice = err.Error()
		}
		return
	}
	state.openDocumentBatch(paths, func(path string) error {
		return state.space.OpenInteractiveWithEncoding(path, encoding)
	})
}

func (state *model) save(index int, saveAs bool) bool {
	doc := state.space.Documents[index]
	state.cancelFileCommandComposition(doc)
	path := doc.Path
	choosingPath := saveAs || path == ""
	if choosingPath {
		defer state.requestNativeChooserFocus()
		chosen, err := app.SaveFile(app.SaveFileOptions{Title: "Save As", SuggestedName: doc.Name, DefaultExtension: "txt"})
		if err != nil {
			if !errors.Is(err, app.ErrCanceled) {
				state.notice = err.Error()
			}
			return false
		}
		path = chosen
	}
	absolute, err := document.CanonicalSavePath(path)
	if err != nil {
		state.notice = err.Error()
		return false
	}
	if state.isPrivateDataPath(absolute) {
		state.notice = "Save destination is an internal VivaPad data file"
		return false
	}
	var target os.FileInfo
	if choosingPath {
		target, _ = os.Stat(absolute)
	}
	for _, other := range state.space.Documents {
		if other != doc && other.Path != "" && destinationMatchesFile(absolute, target, other.Path) {
			state.notice = "This file is already open in another tab"
			return false
		}
	}
	if err := doc.Save(path); err != nil {
		if errors.Is(err, os.ErrPermission) {
			state.notice = "Save failed: permission denied"
		} else {
			state.notice = err.Error()
		}
		return false
	}
	state.notice = "Saved " + filepath.Base(doc.Path)
	state.stopMonitorAfterPathChange(doc)
	state.rememberRecent(doc)
	return true
}

func (state *model) saveCopy(index int) bool {
	doc := state.space.Documents[index]
	state.cancelFileCommandComposition(doc)
	defer state.requestNativeChooserFocus()
	path, err := app.SaveFile(app.SaveFileOptions{Title: "Save a Copy As", SuggestedName: doc.Name, DefaultExtension: "txt"})
	if err != nil {
		if !errors.Is(err, app.ErrCanceled) {
			state.notice = err.Error()
		}
		return false
	}
	absolute, err := document.CanonicalSavePath(path)
	if err != nil {
		state.notice = err.Error()
		return false
	}
	if state.isPrivateDataPath(absolute) {
		state.notice = "Copy destination is an internal VivaPad data file"
		return false
	}
	target, _ := os.Stat(absolute)
	for _, open := range state.space.Documents {
		if open.Path != "" && destinationMatchesFile(absolute, target, open.Path) {
			state.notice = "Copy destination is already open in a tab"
			return false
		}
	}
	if err := doc.SaveCopy(absolute); err != nil {
		state.notice = err.Error()
		return false
	}
	state.notice = "Copied to " + filepath.Base(absolute)
	return true
}

func (state *model) closeTab(index int) {
	closed := state.space.Documents[index]
	if err := state.space.Close(index, false); errors.Is(err, workspace.ErrDirty) {
		state.pendingClose = index
	} else if err != nil {
		state.notice = err.Error()
	} else {
		state.recordClosed(closed)
		state.releaseDocument(closed)
		state.rebindClosedDocument(closed)
	}
}

func (state *model) releaseDocument(doc *document.Document) {
	state.releaseDocumentComposition(doc)
	state.stopMonitor(doc)
	// A tab strip may not paint again while its view is hidden. Rebuild its
	// scratch list on demand instead of keeping a closed document alive.
	state.paneTabScratch = [2][]*document.Document{}
	delete(state.tabHitIndices, doc)
	for pane, anchor := range state.paneTabAnchor {
		if anchor == doc {
			state.paneTabAnchor[pane] = nil
		}
	}
	delete(state.editors, doc)
	delete(state.secondEditors, doc)
	delete(state.paneOwner, doc)
	delete(state.languages, doc)
	delete(state.goHighlighters, doc)
	delete(state.json5Highlighters, doc)
	delete(state.innoHighlighters, doc)
	delete(state.asn1Highlighters, doc)
	delete(state.kixHighlighters, doc)
	delete(state.csoundHighlighters, doc)
	delete(state.rebolHighlighters, doc)
	delete(state.avsHighlighters, doc)
	delete(state.freeBasicHighlighters, doc)
	delete(state.oscriptHighlighters, doc)
	delete(state.pureBasicHighlighters, doc)
	delete(state.gui4CliHighlighters, doc)
	delete(state.txt2tagsHighlighters, doc)
	delete(state.hollywoodHighlighters, doc)
	delete(state.visualPrologHighlighters, doc)
	delete(state.escriptHighlighters, doc)
	delete(state.baanHighlighters, doc)
	delete(state.serverPageHighlighters, doc)
	delete(state.resourceHighlighters, doc)
	delete(state.ansiBackgrounds, doc)
	delete(state.bookmarks, doc)
	delete(state.changeView, doc)
	if state.noChangeNoticeDoc == doc {
		state.retireNoChangeNotice("")
	}
	for pane := range state.multiSelections {
		if state.multiSelections[pane].doc == doc {
			state.multiSelections[pane] = multiSelectionState{}
		}
	}
	delete(state.folded, doc)
	delete(state.foldCache, doc)
	delete(state.hiddenLines, doc)
	delete(state.hiddenValue, doc)
	delete(state.functionSymbols, doc)
	if state.documentListAnchor == doc {
		state.documentListAnchor = nil
	}
	for _, ids := range state.documentListRowIDs {
		delete(ids, doc)
	}
	delete(state.windowsSelected, doc)
	delete(state.windowsRowIDs, doc)
	if state.windowsFocused == doc {
		state.windowsFocused = nil
	}
	if state.functionPaintDoc == doc {
		state.functionPaintDoc, state.functionPaintValue, state.functionPaintMode = nil, "", ""
	}
	delete(state.readOnly, doc)
	delete(state.pinned, doc)
	delete(state.tabColors, doc)
	for pane := range state.selectAnchors {
		if state.selectAnchors[pane].doc == doc {
			state.selectAnchors[pane] = selectAnchor{}
		}
		if state.rectangles[pane].doc == doc {
			state.rectangles[pane] = rectangleSelection{}
			if state.rectangleDragging && state.rectangleDragPane == pane {
				state.rectangleDragging = false
			}
		}
		for _, transition := range state.rectangleHistory[pane] {
			if transition.doc == doc {
				state.rectangleHistory[pane] = nil
				state.rectangleHistoryIndex[pane] = 0
				break
			}
		}
		for _, transition := range state.multiSelectionHistory[pane] {
			if transition.doc == doc {
				state.multiSelectionHistory[pane] = nil
				state.multiSelectionHistoryIndex[pane] = 0
				break
			}
		}
	}
	if state.marks.doc == doc {
		state.marks = searchMarkCache{}
	}
	if state.markedDocument == doc {
		state.markedDocument, state.markedValue, state.markedSpans = nil, "", nil
		state.markedTracking = indicatorTracker{}
	}
	delete(state.styledMarks, doc)
	if state.resultDocument == doc {
		state.discardFileResults()
	}
	if state.resultOpenDocuments {
		matches := state.fileResults.Matches[:0]
		sources := state.resultDocuments[:0]
		previousCount := len(state.resultDocuments)
		removedBefore, selectedRemoved := 0, false
		for index, source := range state.resultDocuments {
			if source != doc {
				matches = append(matches, state.fileResults.Matches[index])
				sources = append(sources, source)
			} else {
				if index < state.resultSelected {
					removedBefore++
				}
				selectedRemoved = selectedRemoved || index == state.resultSelected
			}
		}
		if selectedRemoved {
			state.resultSelected = -1
		} else if state.resultSelected >= 0 {
			state.resultSelected -= removedBefore
		}
		for index := len(matches); index < len(state.fileResults.Matches); index++ {
			state.fileResults.Matches[index] = searchfiles.Match{}
			state.resultDocuments[index] = nil
		}
		state.fileResults.Matches, state.resultDocuments = matches, sources
		state.resultOffset = min(state.resultOffset, max(0, len(matches)-1))
		if len(sources) < previousCount {
			if len(sources) == 0 {
				state.discardFileResults()
				state.notice = "No search results remain"
			} else if state.fileResults.Truncated {
				state.notice = strconv.Itoa(len(matches)) + " matches remain from a truncated search"
			} else {
				state.notice = strconv.Itoa(len(matches)) + " matches remain in open documents"
			}
		}
	}
}

func (state *model) finishClose(discard bool) {
	index := state.pendingClose
	if index < 0 {
		return
	}
	if !discard && !state.save(index, false) {
		return
	}
	closed := state.space.Documents[index]
	if err := state.space.Close(index, true); err != nil {
		state.notice = err.Error()
		return
	}
	if state.allowClose != nil && state.sessionPath != "" && closed.Path != "" {
		if state.closingTabSettings == nil {
			state.closingTabSettings = make(map[*document.Document]sessionTabSettings)
		}
		state.closingTabSettings[closed] = state.sessionSettingsForTab(closed)
	}
	state.recordClosed(closed)
	state.releaseDocument(closed)
	state.rebindClosedDocument(closed)
	state.pendingClose = -1
	if state.allowClose != nil {
		allow := state.allowClose
		state.allowClose = nil
		for next, doc := range state.space.Documents {
			if doc.Dirty() {
				state.pendingClose, state.allowClose = next, allow
				return
			}
		}
		state.persistSession()
		allow()
	} else if state.batchActive {
		state.batchClosed++
		state.continueBatchClose()
	}
}

func (state *model) cancelClose() {
	if state.batchActive {
		state.notice = "Batch close canceled after " + strconv.Itoa(state.batchClosed) + " tab(s)"
		state.batchActive, state.batchQueue, state.batchKeep, state.batchClosed = false, nil, nil, 0
	}
	state.pendingClose, state.allowClose = -1, nil
	state.closingDocs, state.closingActive = nil, nil
	state.closingTabSettings = nil
	state.closingOwners, state.closingPaneDocs = nil, [2]*document.Document{}
	state.closingSplit = false
	state.focusViewRequested, state.focusViewArmed = true, false
	state.invalidator.Invalidate()
}

func (state *model) persistSession() {
	state.closingAccepted = true
	if state.recoveryWriter != nil {
		if err := state.recoveryWriter.Clear(); err != nil {
			state.notice = "Recovery not cleared: " + err.Error()
		}
	}
	if state.sessionPath == "" {
		return
	}
	stored := session.State{}
	stored.FolderRoots = append([]string(nil), state.folderRoots...)
	stored.FolderVisible = state.folderVisible
	stored.DocumentMapVisible = state.documentMapVisible
	stored.FunctionListVisible = state.functionListVisible
	stored.DocumentListVisible = state.documentListVisible
	stored.ActiveLeftDock = state.activeLeftDock
	if stored.ActiveLeftDock == "characters" {
		stored.ActiveLeftDock = ""
	}
	for _, doc := range state.closingDocs {
		if doc.Path != "" {
			stored.Paths = append(stored.Paths, doc.Path)
			settings, closed := state.closingTabSettings[doc]
			if !closed {
				settings = state.sessionSettingsForTab(doc)
			}
			if len(settings.folds.Headers) != 0 {
				if stored.Folds == nil {
					stored.Folds = make(map[string]session.FoldState)
				}
				stored.Folds[doc.Path] = settings.folds
			}
			if settings.languageSelected {
				if stored.Languages == nil {
					stored.Languages = make(map[string]string)
				}
				stored.Languages[doc.Path] = settings.language
			}
			if settings.readOnly {
				if stored.ReadOnly == nil {
					stored.ReadOnly = make(map[string]bool)
				}
				stored.ReadOnly[doc.Path] = true
			}
			if settings.pinned {
				if stored.Pinned == nil {
					stored.Pinned = make(map[string]bool)
				}
				stored.Pinned[doc.Path] = true
			}
			if doc.RTL {
				if stored.RTL == nil {
					stored.RTL = make(map[string]bool)
				}
				stored.RTL[doc.Path] = true
			}
			if settings.color > 0 {
				if stored.TabColors == nil {
					stored.TabColors = make(map[string]uint8)
				}
				stored.TabColors[doc.Path] = settings.color
			}
			if spec, ok := doc.SavedEncoding().LegacySpec(); ok {
				if stored.Encodings == nil {
					stored.Encodings = make(map[string]string)
				}
				stored.Encodings[doc.Path] = spec.Label
			}
		}
	}
	if state.closingActive != nil {
		stored.Active = state.closingActive.Path
	}
	if stored.Active != "" {
		found := false
		for _, path := range stored.Paths {
			found = found || path == stored.Active
		}
		if !found {
			stored.Active = ""
		}
	}
	if state.closingSplit {
		stored.View = session.ViewState{Split: true, ActivePane: state.closingActivePane, Divider: state.closingDivider}
		if doc := state.closingPaneDocs[0]; doc != nil {
			stored.View.Left = doc.Path
		}
		if doc := state.closingPaneDocs[1]; doc != nil {
			stored.View.Right = doc.Path
		}
		for _, doc := range state.closingDocs {
			if doc.Path != "" && state.closingOwners[doc] == 1 {
				stored.View.RightPaths = append(stored.View.RightPaths, doc.Path)
			}
		}
	}
	if err := session.Save(state.sessionPath, stored); err != nil {
		state.notice = "Session not saved: " + err.Error()
	}
	state.closingDocs, state.closingActive = nil, nil
	state.closingTabSettings = nil
	state.closingOwners, state.closingPaneDocs = nil, [2]*document.Document{}
}

type sessionTabSettings struct {
	folds            session.FoldState
	language         string
	languageSelected bool
	readOnly, pinned bool
	color            uint8
}

func (state *model) sessionSettingsForTab(doc *document.Document) sessionTabSettings {
	mode, selected := state.languages[doc]
	settings := sessionTabSettings{
		language: mode, languageSelected: selected,
		readOnly: state.readOnly[doc], pinned: state.isPinned(doc), color: state.tabColors[doc],
	}
	// Fold rows from a discarded draft cannot be paired with the saved digest.
	if !doc.Dirty() {
		settings.folds = session.FoldState{Digest: doc.DiskDigest(), Headers: state.foldHeaders(doc)}
	}
	return settings
}

func (state *model) find(backward bool) {
	if state.query.String() == "" {
		state.notice = "Enter text to find"
		return
	}
	found, err := state.space.Current().FindWith(state.query.String(), state.searchOptions, backward)
	if err != nil {
		state.notice = err.Error()
	} else if found {
		state.notice = ""
	} else {
		state.notice = "Text not found"
	}
}

func (state *model) searchMarks() []text.Span {
	if !state.searchOpen || state.query.String() == "" {
		return nil
	}
	doc := state.space.Current()
	value, query := doc.Editor.String(), state.query.String()
	if state.marks.doc == doc && state.marks.value == value && state.marks.query == query && state.marks.options == state.searchOptions {
		return state.marks.spans
	}
	spans, err := doc.MatchesWith(query, state.searchOptions)
	if err != nil {
		return nil
	}
	state.marks = searchMarkCache{doc: doc, value: value, query: query, options: state.searchOptions, spans: spans}
	return spans
}

func (state *model) markAll() {
	state.markCurrentDocument()
}

func (state *model) clearMarks() {
	state.markedDocument, state.markedValue, state.markedSpans = nil, "", nil
	state.markedTracking = indicatorTracker{}
	state.notice = "Marks cleared"
}

func (state *model) goToLine() {
	line, err := strconv.Atoi(strings.TrimSpace(state.goToField.String()))
	if err != nil || !state.space.Current().GoToLine(line) {
		state.notice = "Enter a line from 1 to " + strconv.Itoa(state.space.Current().LineCount())
		return
	}
	state.goToOpen, state.notice = false, ""
}

func (state *model) command(command string) {
	// Post-It settles editor state only after the native transition is confirmed.
	if command == postItCommand {
		state.togglePostIt()
		return
	}
	if command == textDirectionRTL || command == textDirectionLTR {
		state.setTextDirection(command == textDirectionRTL)
		return
	}
	doc := state.space.Current()
	fromSearchMenu := state.menuOpen == 2
	// Clear refuses a live composition without mutating anything, so its preflight
	// must run before the generic composition cancel below.
	historyNotice := false
	switch command {
	case "Clear Change History":
		if err := doc.ClearChangeHistoryPreflight(); err != nil {
			// A refusal still dismisses the menus and returns focus, so the user can
			// retry after finishing the composition.
			state.menuOpen, state.changeHistoryMenuOpen, state.menuOverflowOpen = -1, false, false
			state.notice = err.Error()
			state.restoreEditorCommandFocus(command)
			return
		}
		historyNotice = true
	case "Next Change", "Previous Change", "Change History",
		"Change History: Margin", "Change History: Indicator", "Change History: Both", "Change History: Off":
		// Opening the submenu and changing the display mode report through the same
		// notice, which a bookmark synchronization must not overwrite.
		historyNotice = true
	}
	columnHelp := command == columnModeHelpCommand
	checksComposition := command == "Replace All in Open Documents" ||
		command == "Next Search Result" || command == "Previous Search Result" || isCompletionCommand(command) ||
		command == "Auto-Completion" || command == webSearchCommand || isCalltipCommand(command) ||
		command == pasteSpecialCommand || isPasteSourceCommand(command) || columnHelp
	if !checksComposition {
		state.cancelLiveComposition()
	}
	// Batch replacement and result navigation validate composition before acting.
	// Do not map bookmarks or selection anchors against a provisional buffer.
	trackActiveSource := !columnHelp && !isPasteSourceCommand(command) && (!checksComposition || !doc.Editor.CompositionActive())
	if trackActiveSource {
		state.bookmarkLines(doc)
	}
	before := doc.Editor.String()
	editStart := doc.Editor.Selection().Start
	defer func() {
		notice := state.notice
		if trackActiveSource {
			state.bookmarkLines(doc)
			state.relocateSelectAnchors(doc, before, doc.Editor.String(), editStart)
		}
		if historyNotice {
			// Bookmark synchronisation has its own warnings; a change-history command
			// owns its result (success, refusal or unavailable reason).
			state.notice = notice
		}
		state.restoreEditorCommandFocus(command)
	}()
	state.menuOpen = -1
	state.recentOpen, state.lineMenuOpen, state.caseMenuOpen, state.sessionMenuOpen, state.bookmarkMenuOpen, state.changeHistoryMenuOpen, state.closeMultipleMenuOpen, state.containingMenuOpen, state.autoMenuOpen, state.selectedSearchMenuOpen, state.blankMenuOpen, state.copyPathsMenuOpen, state.readOnlyMenuOpen, state.onSelectionMenuOpen, state.multiAllMenuOpen, state.multiNextMenuOpen = false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false
	state.insertDateOpen = false
	state.pasteSpecialOpen = false
	state.markJumpMenuOpen = false
	state.styleMenuMode = ""
	state.hashMenuOpen = false
	state.symbolMenuOpen = false
	state.viewTabMenuOpen = false
	state.namedBrowserMenuOpen = false
	state.foldMenuOpen = false
	state.windowSortMenuOpen = false
	state.commentMenuOpen = false
	state.eolMenuOpen = false
	state.charsetMenuOpen = false
	state.savedMacroMenuOpen = false
	if !isCompletionCommand(command) {
		state.wordCompletion = nil
		state.wordFocusPending = false
	}
	if state.isReadOnly(doc) && state.textMutation(command) {
		state.notice = "Read-only document"
		return
	}
	if all, options, ok := multiSelectModeForCommand(command); ok {
		state.multiSelect(options, all)
		step := macro.Step{Kind: macro.SelectionStep, Selection: macro.SelectNextMatch}
		if all {
			step.Selection = macro.SelectAllMatches
		}
		if options.MatchCase {
			step.SelectFlags |= macro.SelectMatchCase
		}
		if options.WholeWord {
			step.SelectFlags |= macro.SelectWholeWord
		}
		state.recordMacro(doc, step)
		state.focusViewRequested, state.focusViewArmed = true, false
		state.invalidator.Invalidate()
		return
	}
	if state.macroRecording && !columnHelp && !macroMenuSupported(command) && !isCalltipCommand(command) {
		state.cancelMacroUnsupported("command")
	}
	if language, ok := languageExtension(command); ok {
		state.languages[doc] = language
		state.configureCode(doc)
		return
	}
	if foldLevelForCommand(command) {
		state.foldCommand(doc, command)
		return
	}
	if sortCommand, ok := windowSortFor(command); ok {
		if err := state.sortWindowTabs(sortCommand); err != nil {
			state.notice = "Window sort failed: " + err.Error()
		} else {
			state.notice = command
		}
		return
	}
	if encoding, ok := document.EncodingFromLabel(command); ok {
		doc.SetEncoding(encoding)
		return
	}
	if operation, ok := blankOperationForCommand(command); ok {
		if state.applyBlankOperation(doc, operation) {
			state.notice = command
		} else {
			state.notice = "No blank changes"
		}
		return
	}
	if isLineSortCommand(command) && state.hasMultipleStreamSelections(doc) {
		state.notice = "Multiple stream selections cannot be sorted"
		return
	}
	if state.sortRectangleCommand(doc, command) {
		return
	}
	if mode, ascending, ok := numericSortForCommand(command); ok {
		changed := false
		var sortError error
		state.editLineTargetsWithBookmarks(doc, "replace", func() { changed, sortError = doc.SortLinesMode(mode, ascending) })
		if sortError != nil {
			state.notice = sortError.Error()
		} else if changed {
			state.notice = command
		} else {
			state.notice = "Lines already sorted"
		}
		return
	}
	switch command {
	case "New":
		state.showInActivePane(state.space.NewDocument())
	case "Open...":
		state.openFiles()
	case "Open Folder as Workspace...":
		state.chooseFolderRoot()
	case "Folder of Current Document as Workspace":
		state.addCurrentDocumentFolder()
	case "Open Containing Folder":
		state.menuOpen, state.containingMenuOpen = 0, true
	case "Explorer":
		state.revealCurrentFile()
	case openCommandPromptCommand:
		state.openCurrentConsole(external.CommandPrompt, "Command Prompt")
	case openPowerShellCommand:
		state.openCurrentConsole(external.WindowsPowerShell, "PowerShell")
	case "Open in Default Viewer":
		state.openDefaultViewer()
	case "Open File from Selection":
		state.openSelectedPath(false)
	case "Open Containing Folder from Selection":
		state.openSelectedPath(true)
	case "Auto-Completion":
		state.menuOpen, state.autoMenuOpen = 1, true
	case "Word Completion":
		state.startWordCompletion()
	case "Path Completion":
		state.startPathCompletion()
	case functionCompletionCommand:
		state.startFunctionCompletion()
	case parameterHintCommand:
		state.startCalltip()
	case previousHintCommand:
		state.cycleCalltip(-1)
	case nextHintCommand:
		state.cycleCalltip(1)
	case "Folder as Workspace":
		state.folderVisible = !state.folderVisible
		if !state.folderVisible {
			state.stopFolderWatching()
			if state.functionListVisible {
				state.activeLeftDock = "functions"
			} else if state.documentListVisible {
				state.activeLeftDock = "documents"
			} else {
				state.activeLeftDock = ""
			}
		} else {
			state.activeLeftDock = "workspace"
		}
	case "Function List":
		state.functionListVisible = !state.functionListVisible
		if state.functionListVisible {
			state.activeLeftDock, state.functionFilterFocus = "functions", true
		} else if state.folderVisible {
			state.activeLeftDock = "workspace"
		} else if state.documentListVisible {
			state.activeLeftDock = "documents"
		} else {
			state.activeLeftDock = ""
		}
	case "Switch to Function List":
		state.functionListVisible, state.activeLeftDock, state.functionFilterFocus = true, "functions", true
	case "Document List":
		state.documentListVisible = !state.documentListVisible
		if state.documentListVisible {
			state.activeLeftDock, state.documentListFocusRequested = "documents", true
		} else if state.folderVisible {
			state.activeLeftDock = "workspace"
		} else if state.functionListVisible {
			state.activeLeftDock = "functions"
		} else {
			state.activeLeftDock = ""
		}
	case "Switch to Document List":
		state.documentListVisible, state.activeLeftDock, state.documentListFocusRequested = true, "documents", true
	case "Document Map":
		state.documentMapVisible = !state.documentMapVisible
		state.documentMapDragging = false
	case "Summary...":
		summaryDoc := doc
		if state.splitEnabled {
			summaryDoc = state.paneDocument(state.activePane)
		}
		state.summary, state.summaryOpen = state.summaryForPane(summaryDoc, state.activePane), true
	case aboutHelpCommand:
		state.openHelp(aboutHelp)
	case debugHelpCommand:
		state.openHelp(debugHelp)
	case projectPageHelpCommand, userGuideHelpCommand, commandLineHelpCommand, columnModeHelpCommand:
		state.openHelpLink(command, app.OpenURL)
	case "Monitoring (tail -f)":
		state.toggleMonitoring()
	case "Synchronize Vertical Scrolling":
		state.syncVertical = !state.syncVertical
		if state.syncVertical {
			state.captureSyncScroll()
		}
	case "Synchronize Horizontal Scrolling":
		state.syncHorizontal = !state.syncHorizontal
		if state.syncHorizontal {
			state.captureSyncScroll()
		}
	case "Sort By":
		state.menuOpen, state.windowSortMenuOpen = 11, true
	case "Windows...":
		state.openWindowsDialog()
	case "Open as GB18030...":
		state.openFilesWithEncoding(document.GB18030)
	case "Open as Windows-1252...":
		state.openFilesWithEncoding(document.Windows1252)
	case "Open as Encoding...":
		state.menuOpen, state.charsetMenuOpen, state.charsetOffset = 0, true, 0
	case "Save":
		state.save(state.space.Active, false)
	case "Save As...":
		state.save(state.space.Active, true)
	case "Save a Copy As...":
		state.saveCopy(state.space.Active)
	case "Rename...":
		state.requestRename()
	case "Save All":
		state.requestEditorCommandFocus()
		for index, openDocument := range state.space.Documents {
			if openDocument.Dirty() && !state.save(index, false) {
				break
			}
		}
	case "Close":
		state.closePaneTab(state.activePane, state.paneDocument(state.activePane))
	case "Close All":
		state.startBatchClose(nil)
	case "Close Other Tabs":
		state.startBatchClose(state.space.Current())
	case "Close Multiple Documents":
		state.menuOpen, state.closeMultipleMenuOpen = 0, true
	case "Move to Recycle Bin":
		state.requestTrash()
	case "Close All but Active Document":
		state.startBatchClose(state.space.Current())
	case "Close All but Pinned Documents":
		state.startAllButPinned()
	case "Close All to the Left", "Close All to the Right", "Close All Unchanged":
		state.startViewClose(command)
	case "Pin Tab", "Unpin Tab":
		state.togglePinned(state.paneDocument(state.activePane), state.activePane)
	case "Reload from Disk":
		state.requestReload()
	case recentFilesCommand:
		state.menuOpen, state.recentOpen = 0, true
	case "Sessions":
		state.menuOpen, state.sessionMenuOpen = 0, true
	case "Character sets":
		state.menuOpen, state.charsetMenuOpen, state.charsetOffset = 4, true, 0
	case "Load Session...":
		state.loadNPPSession()
	case "Save Session...":
		state.openSaveSessionOptions()
	case "Print...":
		state.printCurrent(true)
	case "Print Now":
		state.printCurrent(false)
	case "Restore Recent Closed File":
		state.restoreClosed()
	case "Undo":
		before := doc.Editor.String()
		if doc.Editor.Undo() {
			state.restoreRectangleAcrossViews(doc, before, doc.Editor.String(), true)
			state.restoreMultiSelectionAcrossViews(doc, before, doc.Editor.String(), true)
		}
		state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextCommandEvent, Command: app.TextUndo})
	case "Redo":
		before := doc.Editor.String()
		if doc.Editor.Redo() {
			state.restoreRectangleAcrossViews(doc, before, doc.Editor.String(), false)
			state.restoreMultiSelectionAcrossViews(doc, before, doc.Editor.String(), false)
		}
		state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextCommandEvent, Command: app.TextRedo})
	case "Cut", "Copy":
		if state.multiSelections[state.activePane].active && state.multiSelections[state.activePane].doc == doc {
			state.copyMultiSelection(doc, command == "Cut")
			break
		}
		if selection := state.rectangles[state.activePane]; selection.active && selection.doc == doc && selection.value == doc.Editor.String() {
			state.copyRectangle(doc, command == "Cut")
			break
		}
		selection := doc.Editor.Selection()
		value := []rune(doc.Editor.String())
		if selection.Start != selection.End {
			if err := app.SetClipboardText(string(value[selection.Start:selection.End])); err != nil {
				state.notice = err.Error()
			} else if command == "Cut" {
				state.editWithBookmarkAnchor(doc, func() { doc.Editor.Insert("") })
				state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextCommandEvent, Command: app.TextDeleteForward})
			}
		}
	case pasteSpecialCommand:
		state.menuOpen, state.pasteSpecialOpen = 1, true
	case pasteHTMLCommand, pasteRTFCommand:
		state.pasteClipboardSource(command)
	case "Paste":
		value, rectangular, err := app.ClipboardTextWithFormat()
		if err != nil {
			state.notice = err.Error()
		} else if state.multiSelections[state.activePane].active && state.multiSelections[state.activePane].doc == doc {
			if rectangular {
				state.notice = "Clear multi-selection before column paste"
			} else if _, err := state.editMultiSelection(state.activePane, value, multiInsert); err != nil {
				state.notice = "Multi-selection paste failed: " + err.Error()
			}
		} else if rectangular {
			if selection := doc.Editor.Selection(); selection.Start != selection.End {
				state.notice = "Clear the selection before column paste"
				break
			}
			state.editWithBookmarkAnchor(doc, func() {
				_, err = doc.PasteRectangleLines(document.Rectangle{Anchor: doc.Editor.Caret(), Caret: doc.Editor.Caret(), TabWidth: state.tabWidth}, value)
			})
			if err != nil {
				state.notice = err.Error()
			} else if state.macroRecording {
				state.cancelMacroUnsupported("column paste")
			}
		} else {
			state.editWithBookmarkAnchor(doc, func() { doc.Editor.Paste(value) })
			state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextPaste, Text: value})
		}
	case "Delete":
		deleted := false
		state.editWithBookmarkAnchor(doc, func() { deleted = doc.Editor.DeleteForward() })
		if deleted {
			state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextCommandEvent, Command: app.TextDeleteForward})
		}
	case "Select All":
		state.multiSelections[state.activePane] = multiSelectionState{}
		doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
		state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextCommandEvent, Command: app.TextSelectAll})
	case "Begin/End Select":
		state.beginEndSelect()
	case "Begin/End Select in Column Mode":
		state.beginEndColumnSelect()
	case "Read-Only in Notepad++":
		state.menuOpen, state.readOnlyMenuOpen = 1, true
	case "Read-Only on Current Document":
		state.readOnly[doc] = !state.readOnly[doc]
		state.notice = "Read-only mode changed"
	case "Read-Only Attribute in Windows", "File Read-Only Attribute", "Toggle File Read-Only Attribute":
		readonly, err := document.ToggleFileReadOnly(doc.Path)
		if err != nil {
			state.notice = "File attribute change failed: " + err.Error()
		} else if readonly {
			state.notice = "File is read-only"
		} else {
			state.notice = "File is writable"
		}
	case "Read-Only for All Documents":
		for _, open := range state.space.Documents {
			state.readOnly[open] = true
		}
		state.notice = "Open documents are read-only"
	case "Clear Read-Only for All Documents":
		clear(state.readOnly)
		state.notice = "Open documents are editable"
	case "Copy to Clipboard":
		state.menuOpen, state.copyPathsMenuOpen = 1, true
	case "On Selection":
		state.menuOpen, state.onSelectionMenuOpen = 1, true
	case webSearchCommand:
		state.searchActiveEditorOnInternet()
	case webSearchSettingsCommand:
		state.openWebSearchSettings()
	case characterPanelCommand:
		state.toggleCharacterPanel()
	case "Multi-select All":
		state.menuOpen, state.multiAllMenuOpen = 1, true
	case "Multi-select Next":
		state.menuOpen, state.multiNextMenuOpen = 1, true
	case "Undo Latest Multi-Select":
		state.undoMultiSelect()
		state.recordMacro(doc, macro.Step{Kind: macro.SelectionStep, Selection: macro.UndoLatestSelection})
		state.focusViewRequested, state.focusViewArmed = true, false
		state.invalidator.Invalidate()
	case "Skip Current Multi-Select":
		state.skipMultiSelect()
		state.recordMacro(doc, macro.Step{Kind: macro.SelectionStep, Selection: macro.SkipCurrentMatch})
		state.focusViewRequested, state.focusViewArmed = true, false
		state.invalidator.Invalidate()
	case "Redact Selection", "Redact Selection with Bullets":
		state.redactSelection(command == "Redact Selection with Bullets")
	case "Copy Current Full File path", "Copy Current Filename", "Copy Current Dir. Path", "Copy All Filenames", "Copy All File Paths":
		state.copyDocumentPaths(command)
	case "Duplicate Line":
		state.applyCodeCommand(doc, components.CodeEditorDuplicateLinesDown)
		state.recordMacro(doc, macro.Step{Kind: macro.LineStep, Line: command})
	case "Delete Line":
		state.deleteLineWithBookmarks(doc)
		state.recordMacro(doc, macro.Step{Kind: macro.LineStep, Line: command})
	case "Indent Lines":
		doc.IndentLines()
		state.recordMacro(doc, macro.Step{Kind: macro.LineStep, Line: command})
	case "Unindent Lines":
		doc.UnindentLines()
		state.recordMacro(doc, macro.Step{Kind: macro.LineStep, Line: command})
	case "Move Up Current Line", "Move Down Current Line":
		state.moveLinesWithBookmarks(doc, command == "Move Up Current Line")
	case "Line Operations":
		state.menuOpen, state.lineMenuOpen, state.lineMenuOffset = 1, true, 0
	case "Insert":
		state.menuOpen, state.insertDateOpen = 1, true
	case "Date Time (short)":
		state.insertDateTime(doc, datetime.Short)
	case "Date Time (long)":
		state.insertDateTime(doc, datetime.Long)
	case "Date Time (customized)":
		state.insertDateTime(doc, datetime.Customized)
	case "Date Time Format...":
		state.openDateTimeDialog()
	case "Show Symbol":
		state.menuOpen, state.symbolMenuOpen = 3, true
	case "Tab":
		state.menuOpen, state.viewTabMenuOpen, state.viewTabOffset = 3, true, 0
	case namedBrowserMenuCommand:
		state.menuOpen, state.namedBrowserMenuOpen = 3, true
	case "Apply Color 1", "Apply Color 2", "Apply Color 3", "Apply Color 4", "Apply Color 5", "Remove Color":
		state.setTabColor(command)
	case "1st Tab", "2nd Tab", "3rd Tab", "4th Tab", "5th Tab", "6th Tab", "7th Tab", "8th Tab", "9th Tab":
		for index := 0; index < 9; index++ {
			if command == viewTabCommands[index] {
				state.selectViewTab(index)
				break
			}
		}
	case "First Tab":
		state.selectViewTab(0)
	case "Last Tab":
		state.selectViewTab(len(state.activeViewTabs()) - 1)
	case "Next Tab":
		state.cycleTabs(false)
	case "Previous Tab":
		state.cycleTabs(true)
	case "Move to Start", "Move to End", "Move Tab Forward", "Move Tab Backward":
		state.moveViewTab(command)
	case "Show Space and Tab":
		state.showWhitespace = !state.showWhitespace
		state.savePreferences()
	case "Show End of Line":
		state.showEOL = !state.showEOL
		state.savePreferences()
	case "Show Indent Guide":
		state.showIndentGuides = !state.showIndentGuides
		state.savePreferences()
	case "Show Wrap Symbol":
		state.showWrapSymbol = !state.showWrapSymbol
		state.savePreferences()
	case "Show Non-Printing Characters":
		state.showNonPrinting = !state.showNonPrinting
		state.representationsReady = false
		state.savePreferences()
	case "Show Control Characters & Unicode EOL":
		state.showControls = !state.showControls
		state.representationsReady = false
		state.savePreferences()
	case "Show All Characters":
		enabled := !(state.showWhitespace && state.showEOL && state.showNonPrinting && state.showControls)
		state.showWhitespace, state.showEOL, state.showNonPrinting, state.showControls = enabled, enabled, enabled, enabled
		state.representationsReady = false
		state.savePreferences()
	case "Non-Printing Mode: Original", "Non-Printing Mode: Abbreviation", "Non-Printing Mode: Codepoint":
		state.npcMode = map[string]string{"Non-Printing Mode: Original": "identity", "Non-Printing Mode: Abbreviation": "abbreviation", "Non-Printing Mode: Codepoint": "codepoint"}[command]
		state.representationsReady = false
		state.savePreferences()
	case "Include Control Characters in NPC Mode":
		state.npcIncludeControls = !state.npcIncludeControls
		state.representationsReady = false
		state.savePreferences()
	case "MD5", "SHA-1", "SHA-256", "SHA-512":
		state.hashAlgorithm = hashAlgorithmForName(command)
		state.hashMenuIndex = int(state.hashAlgorithm) - 1
		state.menuOpen, state.hashMenuOpen = 7, true
	case "Generate...":
		state.openHashDialog(hashFromText)
	case "Generate from files...":
		state.openHashDialog(hashFromFiles)
	case "Generate from selection into clipboard":
		state.hashSelectionToClipboard(doc)
	case "Split Lines":
		if state.splitHasMultipleSelections(doc) {
			state.notice = "Split Lines requires a single selection"
			return
		}
		breaks := state.codeEditorForPane(state.activePane).SoftWrapOffsets(nil)
		changed := false
		state.editLineTargetsWithBookmarks(doc, "split", func() { changed = doc.SplitLinesAt(breaks) })
		if changed {
			state.notice = "Split at visual line wraps"
		} else {
			state.notice = "No visual line breaks to split"
		}
	case "Join Lines":
		changed := false
		state.editLineTargetsWithBookmarks(doc, "join", func() { changed = doc.JoinLines() })
		if changed {
			state.notice = "Joined selected lines"
		} else {
			state.notice = "Select two or more lines to join"
		}
	case "Reverse Line Order":
		changed := false
		state.editLineTargetsWithBookmarks(doc, "replace", func() { changed = doc.ReverseLines() })
		if changed {
			state.notice = "Reversed line order"
		} else {
			state.notice = "Line order unchanged"
		}
	case "Insert Blank Line Above Current":
		state.editLineTargetsWithBookmarks(doc, "replace", doc.InsertBlankLineAbove)
		state.notice = "Inserted blank line above"
	case "Insert Blank Line Below Current":
		state.editLineTargetsWithBookmarks(doc, "replace", doc.InsertBlankLineBelow)
		state.notice = "Inserted blank line below"
	case "Blank Operations":
		state.menuOpen, state.blankMenuOpen = 1, true
	case "Convert Case":
		state.menuOpen, state.caseMenuOpen = 1, true
	case "Comment/Uncomment":
		state.menuOpen, state.commentMenuOpen = 1, true
	case "Toggle Single Line Comment", "Single Line Comment", "Single Line Uncomment", "Block Comment", "Block Uncomment":
		state.applyCommentCommand(doc, command)
	case "UPPERCASE":
		state.convertSelectedCase(doc, document.CaseUpper, "Converted to uppercase")
	case "lowercase":
		state.convertSelectedCase(doc, document.CaseLower, "Converted to lowercase")
	case "Proper Case":
		state.convertSelectedCase(doc, document.CaseProperForce, "Converted to Proper Case")
	case "Proper Case (blend)":
		state.convertSelectedCase(doc, document.CaseProperBlend, "Blended Proper Case")
	case "Sentence case":
		state.convertSelectedCase(doc, document.CaseSentenceForce, "Converted to sentence case")
	case "Sentence case (blend)":
		state.convertSelectedCase(doc, document.CaseSentenceBlend, "Blended sentence case")
	case "Invert Case":
		state.convertSelectedCase(doc, document.CaseInvert, "Inverted case")
	case "Random Case":
		state.convertSelectedCase(doc, document.CaseRandom, "Randomized case")
	case "Sort Lines Ascending":
		changed := false
		state.editLineTargetsWithBookmarks(doc, "replace", func() { changed = doc.SortLines(true) })
		if changed {
			state.notice = "Sorted lines ascending"
		} else {
			state.notice = "Lines already sorted"
		}
	case "Sort Lines Descending":
		changed := false
		state.editLineTargetsWithBookmarks(doc, "replace", func() { changed = doc.SortLines(false) })
		if changed {
			state.notice = "Sorted lines descending"
		} else {
			state.notice = "Lines already sorted"
		}
	case "Sort In Locale Order Ascending", "Sort In Locale Order Descending":
		changed := false
		var sortError error
		state.editLineTargetsWithBookmarks(doc, "replace", func() {
			changed, sortError = doc.SortLinesLocaleWithError(preferredSortLanguage(), command == "Sort In Locale Order Ascending")
		})
		if sortError != nil {
			state.notice = sortError.Error()
		} else if changed {
			state.notice = command
		} else {
			state.notice = "Lines already sorted"
		}
	case "Change History: Margin":
		state.setChangeHistoryView("margin")
	case "Change History: Indicator":
		state.setChangeHistoryView("indicator")
	case "Change History: Both":
		state.setChangeHistoryView("both")
	case "Change History: Off":
		state.setChangeHistoryView("off")
	case "Next Change":
		state.navigateChange(false)
	case "Previous Change":
		state.navigateChange(true)
	case "Clear Change History":
		state.clearChangeHistory()
	case "Randomize Line Order":
		changed, randomizeErr := state.randomizeLineOrder(doc, uint64(time.Now().Unix()))
		if randomizeErr != nil {
			state.notice = randomizeErr.Error()
		} else if changed {
			state.notice = "Randomized line order"
		} else {
			state.notice = "Line order unchanged"
		}
	case "Remove Duplicate Lines":
		removed := 0
		state.editLineTargetsWithBookmarks(doc, "replace", func() { removed = doc.RemoveDuplicateLines() })
		state.notice = strconv.Itoa(removed) + " duplicate line(s) removed"
	case "Sort Ascending (Ignore Case)":
		changed := false
		state.editLineTargetsWithBookmarks(doc, "replace", func() { changed = doc.SortLinesIgnoreCase(true) })
		if changed {
			state.notice = "Sorted lines ignoring case"
		} else {
			state.notice = "Lines already sorted"
		}
	case "Sort Descending (Ignore Case)":
		changed := false
		state.editLineTargetsWithBookmarks(doc, "replace", func() { changed = doc.SortLinesIgnoreCase(false) })
		if changed {
			state.notice = "Sorted lines descending ignoring case"
		} else {
			state.notice = "Lines already sorted"
		}
	case "Remove Empty Lines":
		removed := 0
		state.editLineTargetsWithBookmarks(doc, "delete", func() { removed = doc.RemoveEmptyLines(false) })
		state.notice = strconv.Itoa(removed) + " empty line(s) removed"
	case "Remove Blank Lines":
		removed := 0
		state.editLineTargetsWithBookmarks(doc, "delete", func() { removed = doc.RemoveEmptyLines(true) })
		state.notice = strconv.Itoa(removed) + " blank line(s) removed"
	case "Remove Consecutive Duplicates":
		removed := 0
		state.editLineTargetsWithBookmarks(doc, "delete", func() { removed = doc.RemoveConsecutiveDuplicateLines() })
		state.notice = strconv.Itoa(removed) + " consecutive duplicate line(s) removed"
	case "Record Macro":
		state.startMacro()
	case "Stop Recording":
		state.stopMacro()
	case "Playback Macro":
		state.playMacro(1)
	case "Save Current Recorded Macro...":
		state.openSaveMacroDialog()
	case "Run a Macro Multiple Times...":
		state.openRunMacroDialog()
	case "Saved Macros":
		state.menuOpen, state.savedMacroMenuOpen, state.savedMacroOffset = 8, true, 0
	case "Playback 5 Times":
		state.playMacro(5)
	case "Clear Macro":
		state.clearMacro()
	case "Show Line Numbers":
		state.showLineNumbers = !state.showLineNumbers
		state.savePreferences()
	case "Word Wrap":
		state.wrap = !state.wrap
		state.savePreferences()
	case "Tab Width: 2", "Tab Width: 4", "Tab Width: 8":
		state.tabWidth = int(command[len(command)-1] - '0')
		state.savePreferences()
	case "Indent: Language Default":
		state.indentMode = "language"
		state.savePreferences()
	case "Indent: Tabs":
		state.indentMode = "tabs"
		state.savePreferences()
	case "Indent: Spaces":
		state.indentMode = "spaces"
		state.savePreferences()
	case "Auto Indent":
		state.autoIndent = !state.autoIndent
		state.savePreferences()
	case "Pair Brackets":
		state.autoClose = !state.autoClose
		state.savePreferences()
	case "Print Line Numbers":
		state.printSettings.LineNumbers = !state.printSettings.LineNumbers
		state.savePreferences()
	case "Print Form Feed":
		state.printSettings.FormFeedPageBreak = !state.printSettings.FormFeedPageBreak
		state.savePreferences()
	case "Preferences...":
		state.preferencesOpen, state.preferencesPage = true, 0
		state.openPrintPreferenceFields()
	case "Column Editor...":
		state.openColumnDialog()
	case "Split View":
		state.toggleSplit()
	case "Focus on Another View":
		if state.splitEnabled {
			state.activatePane(1 - state.activePane)
		}
		state.focusViewRequested, state.focusViewArmed = true, false
		state.invalidator.Invalidate()
	case "Fold All", "Unfold All", "Fold Current Level", "Unfold Current Level":
		state.foldCommand(doc, command)
	case "Hide Lines":
		state.hideSelectedLines(state.paneDocument(state.activePane))
	case "Exit":
		if state.closeWindow == nil {
			state.notice = "Window close unavailable"
		} else {
			state.closeWindow()
		}
	case "Toggle Full Screen Mode":
		if state.postIt.active || state.postIt.pending {
			break
		}
		if state.requestFullscreen == nil {
			state.notice = "Full screen unavailable"
		} else if err := state.requestFullscreen(!state.fullscreen); err != nil {
			state.notice = "Full screen unavailable: " + err.Error()
		}
	case distractionFreeCommand:
		if !state.postIt.active && !state.postIt.pending {
			state.toggleDistractionFree()
		}
	case "Always on Top":
		if state.postIt.active || state.postIt.pending || state.postIt.raised {
			break
		}
		if state.requestAlwaysOnTop == nil {
			state.notice = "Always on Top unavailable"
		} else if err := state.requestAlwaysOnTop(!state.alwaysOnTop); err != nil {
			state.notice = "Always on Top unavailable: " + err.Error()
		} else {
			state.alwaysOnTop = !state.alwaysOnTop
		}
	case "Run...":
		state.openRunDialog()
	case "Fold Level", "Unfold Level":
		state.menuOpen, state.foldMenuOpen, state.foldMenuUnfold = 3, true, command == "Unfold Level"
	case "Move to Other View":
		state.moveToOtherView(false)
	case "Clone to Other View":
		state.moveToOtherView(true)
	case "Zoom In":
		state.zoom(command)
	case "Zoom Out":
		state.zoom(command)
	case "Restore Default Zoom":
		state.zoom(command)
	case "Synchronize Zoom Across Views":
		state.toggleZoomSync()
	case "Find...", "Replace...":
		state.closeIncrementalSearch()
		state.markDialogOpen = false
		state.searchOpen, state.goToOpen = true, false
	case "Find characters in range...":
		state.openByteRangeDialog()
	case "Incremental Search":
		state.openIncrementalSearch(false)
	case "Find Next":
		state.find(false)
	case "Find Previous":
		state.find(true)
	case "Selected Text Search":
		state.menuOpen, state.selectedSearchMenuOpen = 2, true
	case "Select and Find Next":
		state.selectedFind(false, false)
	case "Select and Find Previous":
		state.selectedFind(true, false)
	case "Find (Volatile) Next":
		state.selectedFind(false, true)
	case "Find (Volatile) Previous":
		state.selectedFind(true, true)
	case "Next Search Result", "Previous Search Result":
		state.navigateFileResult(command == "Previous Search Result")
		if fromSearchMenu {
			state.resultFocusMode = false
			state.requestNativeChooserFocus()
		}
	case "Search Results Window":
		if !state.resultsOpen || len(state.fileResults.Matches) == 0 {
			state.notice = "No search results"
		} else {
			state.searchOpen, state.goToOpen = false, false
			state.resultReturnToEditor = state.resultFocusMode
			state.resultFocusMode = !state.resultReturnToEditor
			state.resultFocusRequested = true
		}
	case "Find All in Current Document":
		state.findAllCurrent()
	case "Find All in Open Documents":
		state.findAllOpenDocuments()
	case "Mark...":
		state.markDialogOpen, state.searchOpen, state.goToOpen, state.filesDialogOpen = true, false, false, false
		state.markFocusPending = true
		if state.searchOptions.Regex {
			state.markMode = "regex"
		}
	case "Mark All":
		state.markAll()
	case "Jump Up", "Jump Down":
		state.menuOpen, state.markJumpMenuOpen = 2, true
		state.markJumpPrevious = command == "Jump Up"
	case "Style All Occurrences of Token":
		state.menuOpen, state.styleMenuMode = 2, "all"
	case "Style One Token":
		state.menuOpen, state.styleMenuMode = 2, "one"
	case "Clear Style":
		state.menuOpen, state.styleMenuMode = 2, "clear"
	case "Copy Styled Text":
		state.menuOpen, state.styleMenuMode = 2, "copy"
	case "Previous Find Mark", "Next Find Mark":
		state.navigateMarks(command == "Previous Find Mark")
	case "Clear Marks":
		state.clearMarks()
	case "Toggle Bookmark":
		state.toggleBookmark()
	case "Next Bookmark":
		state.navigateBookmark(false)
	case "Previous Bookmark":
		state.navigateBookmark(true)
	case "Clear All Bookmarks":
		delete(state.bookmarks, doc)
		state.notice = "Bookmarks cleared"
	case "Inverse Bookmarks":
		state.inverseBookmarks()
	case "Copy Bookmarked Lines":
		state.copyBookmarkedLines(false)
	case "Cut Bookmarked Lines":
		state.copyBookmarkedLines(true)
	case "Paste to (Replace) Bookmarked Lines":
		state.pasteToBookmarkedLines()
	case "Remove Bookmarked Lines":
		state.removeBookmarkedLines(true)
	case "Remove Non-Bookmarked Lines":
		state.removeBookmarkedLines(false)
	case "Bookmark":
		state.menuOpen, state.bookmarkMenuOpen = 2, true
	case "Change History":
		state.menuOpen, state.changeHistoryMenuOpen = 2, true
		if reason, bad := changeHistoryUnavailable(doc); bad {
			// Opening the submenu tells the user why its entries cannot act.
			state.notice = reason
		}
	case "Replace All in Open Documents":
		state.replaceAllOpenDocuments()
	case "Find in Files...":
		state.openFilesDialog()
		state.filesReplaceMode = false
	case "Replace in Files...":
		state.openFilesDialog()
		state.filesReplaceMode = true
	case "Go to Line...":
		state.goToOpen, state.searchOpen = true, false
	case "Go to Matching Brace":
		state.matchBrace(false)
	case "Select All In-between {} [] or ()":
		state.matchBrace(true)
	case "Unix (LF)":
		doc.SetLineEnding(document.LF)
	case "Windows (CR LF)":
		doc.SetLineEnding(document.CRLF)
	case "Macintosh (CR)":
		doc.SetLineEnding(document.CR)
	case "EOL Conversion":
		state.menuOpen, state.eolMenuOpen = 1, true
	case "UTF-8":
		doc.SetUTF8BOM(false)
	case "UTF-8 BOM":
		doc.SetUTF8BOM(true)
	case "UTF-16 LE":
		doc.SetEncoding(document.UTF16LE)
	case "UTF-16 BE":
		doc.SetEncoding(document.UTF16BE)
	case "GB18030":
		doc.SetEncoding(document.GB18030)
	case "Windows-1252":
		doc.SetEncoding(document.Windows1252)
	}
}

func languageExtension(name string) (string, bool) {
	if spec, ok := syntax.LanguageByLabel(name); ok {
		return spec.Mode, true
	}
	return "", false
}

func (state *model) openArguments(paths []string, encodings map[string]string) {
	for _, path := range paths {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			state.notice = "Cannot open " + path
			continue
		}
		if err := state.openRemembered(path, encodings); err != nil {
			state.notice = err.Error()
		} else {
			state.rememberRecent(state.space.Current())
		}
	}
}
