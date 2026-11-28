package shortcuts

const (
	MultiSelectNext = "Multi-select Next"
	MultiSelectAll  = "Multi-select All"
	MultiSelectSkip = "Skip Current Multi-Select"
	MultiSelectUndo = "Undo Latest Multi-Select"
	MultiSelectExit = "Exit Multi-Selection"
	CommandPalette  = "Command Palette"
	QuickSwitcher   = "Quick File Switcher"
)

func Defaults() []Command {
	return []Command{
		{ID: "New", Default: Chord{Primary, "N"}},
		{ID: "Open...", Default: Chord{Primary, "O"}},
		{ID: "Save", Default: Chord{Primary, "S"}},
		{ID: "Save As...", Default: Chord{Primary | Shift, "S"}},
		{ID: "Close", Default: Chord{Primary, "W"}},
		{ID: "Restore Recent Closed File", Default: Chord{Primary | Shift, "T"}},
		{ID: "Undo", Default: Chord{Primary, "Z"}, Fixed: true},
		{ID: "Redo", Default: Chord{Primary | Shift, "Z"}, Aliases: []Chord{{Primary, "Y"}}, Fixed: true},
		{ID: "Cut", Default: Chord{Primary, "X"}, Fixed: true},
		{ID: "Copy", Default: Chord{Primary, "C"}, Fixed: true},
		{ID: "Paste", Default: Chord{Primary, "V"}, Fixed: true},
		{ID: "Select All", Default: Chord{Primary, "A"}, Fixed: true},
		{ID: "Duplicate Line", Default: Chord{Primary, "D"}, Aliases: []Chord{{Alt | Shift, "Down"}}},
		{ID: "Duplicate Lines Up", Default: Chord{Alt | Shift, "Up"}},
		{ID: "Delete Line", Default: Chord{Primary, "L"}, Aliases: []Chord{{Primary | Shift, "K"}}},
		{ID: "Move Up Current Line", Default: Chord{Alt, "Up"}, Aliases: []Chord{{Control | Shift, "Up"}}},
		{ID: "Move Down Current Line", Default: Chord{Alt, "Down"}, Aliases: []Chord{{Control | Shift, "Down"}}},
		{ID: "Indent Lines", Default: Chord{Primary, "]"}},
		{ID: "Unindent Lines", Default: Chord{Primary, "["}},
		{ID: "Toggle Single Line Comment", Default: Chord{Primary, "/"}},
		{ID: "lowercase", Default: Chord{Primary, "U"}},
		{ID: "UPPERCASE", Default: Chord{Primary | Shift, "U"}},
		{ID: "Begin/End Select", Default: Chord{Primary | Shift, "B"}},
		{ID: "Begin/End Select in Column Mode", Default: Chord{Alt | Shift, "B"}},
		{ID: MultiSelectNext, Default: Chord{Primary | Shift, "D"}, OptionalDefault: true},
		{ID: MultiSelectAll, Default: Chord{Primary | Shift, "L"}, OptionalDefault: true},
		{ID: MultiSelectSkip, Default: Chord{Primary | Shift, "J"}, OptionalDefault: true},
		{ID: MultiSelectUndo},
		{ID: MultiSelectExit, Default: Chord{0, "Escape"}, OptionalDefault: true},
		{ID: "Word Completion", Default: Chord{Primary, "Enter"}},
		{ID: "Path Completion", Default: Chord{Primary | Shift, "Space"}},
		{ID: "Find...", Default: Chord{Primary, "F"}},
		{ID: "Replace...", Default: Chord{Primary, "H"}},
		{ID: "Find in Files...", Default: Chord{Primary | Shift, "F"}},
		{ID: "Incremental Search", Default: Chord{Primary | Alt, "I"}},
		{ID: "Go to Line...", Default: Chord{Primary, "G"}},
		{ID: "Go to Matching Brace", Default: Chord{Primary, "B"}},
		{ID: "Select All In-between {} [] or ()", Default: Chord{Primary | Alt, "B"}},
		{ID: "Find Next", Default: Chord{0, "F3"}},
		{ID: "Find Previous", Default: Chord{Shift, "F3"}},
		{ID: "Select and Find Next", Default: Chord{Primary, "F3"}},
		{ID: "Select and Find Previous", Default: Chord{Primary | Shift, "F3"}},
		{ID: "Find (Volatile) Next", Default: Chord{Primary | Alt, "F3"}},
		{ID: "Find (Volatile) Previous", Default: Chord{Primary | Alt | Shift, "F3"}},
		{ID: "Next Search Result", Default: Chord{0, "F4"}},
		{ID: "Previous Search Result", Default: Chord{Shift, "F4"}},
		{ID: "Search Results Window", Default: Chord{0, "F7"}},
		{ID: "Next Bookmark", Default: Chord{0, "F2"}},
		{ID: "Previous Bookmark", Default: Chord{Shift, "F2"}},
		{ID: "Toggle Bookmark", Default: Chord{Primary, "F2"}, Aliases: []Chord{{Control, "F2"}}},
		{ID: "Zoom In", Default: Chord{Primary, "="}, Aliases: []Chord{{Primary | Shift, "="}, {Primary, "+"}, {Primary | Shift, "+"}}},
		{ID: "Zoom Out", Default: Chord{Primary, "-"}},
		{ID: "Restore Default Zoom", Default: Chord{Primary, "0"}},
		{ID: "Next Tab", Default: Chord{Control, "Tab"}},
		{ID: "Previous Tab", Default: Chord{Control | Shift, "Tab"}},
		{ID: "Post-It", Default: Chord{0, "F12"}},
		{ID: "Text Direction RTL", Default: Chord{Control | Alt, "R"}},
		{ID: "Text Direction LTR", Default: Chord{Control | Alt, "L"}},
		{ID: "Next Difference", Default: Chord{Primary, "F8"}},
		{ID: "Previous Difference", Default: Chord{Primary | Shift, "F8"}},
		{ID: "Toggle Macro Recording", Default: Chord{Primary | Shift, "R"}},
		{ID: "Playback Macro", Default: Chord{Primary | Alt, "P"}, OptionalDefault: true},
		{ID: CommandPalette, Default: Chord{Primary | Shift, "P"}, OptionalDefault: true},
		{ID: QuickSwitcher, Default: Chord{Primary, "P"}, OptionalDefault: true},
	}
}
