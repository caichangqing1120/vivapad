package syntax

import "strings"

type Language struct {
	Label       string
	Mode        string
	NPPName     string
	SourceID    string
	Lexer       string
	LineComment string
	IndentAfter string
}

var languages = [...]Language{
	{"Normal Text", "", "normal", "IDM_LANG_TEXT", "", "", ""},
	{"Go", ".go", "go", "IDM_LANG_GOLANG", "Go", "//", "{[("},
	{"JavaScript", ".js", "javascript", "IDM_LANG_JS", "JavaScript", "//", "{[("},
	{"TypeScript", ".ts", "typescript", "IDM_LANG_TYPESCRIPT", "TypeScript", "//", "{[("},
	{"Python", ".py", "python", "IDM_LANG_PYTHON", "Python", "#", ":"},
	{"JSON", ".json", "json", "IDM_LANG_JSON", "JSON", "", "{[("},
	{"HTML", ".html", "html", "IDM_LANG_HTML", "HTML", "", ""},
	{"CSS", ".css", "css", "IDM_LANG_CSS", "CSS", "", "{[("},
	{"Markdown", ".md", "markdown", "", "Markdown", "", ""},
	{"SQL", ".sql", "sql", "IDM_LANG_SQL", "SQL", "--", ""},
	{"C", ".c", "c", "IDM_LANG_C", "C", "//", "{[("},
	{"C++", ".cpp", "cpp", "IDM_LANG_CPP", "C++", "//", "{[("},
	{"C#", ".cs", "cs", "IDM_LANG_CS", "C#", "//", "{[("},
	{"Java", ".java", "java", "IDM_LANG_JAVA", "Java", "//", "{[("},
	{"Rust", ".rs", "rust", "IDM_LANG_RUST", "Rust", "//", "{[("},
	{"Ruby", ".rb", "ruby", "IDM_LANG_RUBY", "Ruby", "#", ""},
	{"PHP", ".php", "php", "IDM_LANG_PHP", "PHP", "//", "{[("},
	{"Shell", ".sh", "bash", "IDM_LANG_BASH", "Bash", "#", ""},
	{"YAML", ".yaml", "yaml", "IDM_LANG_YAML", "YAML", "#", ":"},
	{"XML", ".xml", "xml", "IDM_LANG_XML", "XML", "", ""},
	{"TOML", ".toml", "toml", "IDM_LANG_TOML", "TOML", "#", ""},
	{"Lua", ".lua", "lua", "IDM_LANG_LUA", "Lua", "--", ""},
	{"Swift", ".swift", "swift", "IDM_LANG_SWIFT", "Swift", "//", "{[("},
	{"INI file", ".ini", "ini", "IDM_LANG_INI", "INI", ";", ""},
	{"Makefile", ".mk", "makefile", "IDM_LANG_MAKEFILE", "Makefile", "#", ""},
	{"Diff", ".diff", "diff", "IDM_LANG_DIFF", "Diff", "", ""},
	{"PowerShell", ".ps1", "powershell", "IDM_LANG_POWERSHELL", "PowerShell", "#", ""},
	{"Perl", ".pl", "perl", "IDM_LANG_PERL", "Perl", "#", ""},
	{"R", ".r", "r", "IDM_LANG_R", "R", "#", ""},
	{"Pascal", ".pas", "pascal", "IDM_LANG_PASCAL", "ObjectPascal", "//", ""},
	{"Haskell", ".hs", "haskell", "IDM_LANG_HASKELL", "Haskell", "--", ""},
	{"Erlang", ".erl", "erlang", "IDM_LANG_ERLANG", "Erlang", "%", ""},
	{"LaTeX", ".tex", "latex", "IDM_LANG_LATEX", "LaTeX", "%", ""},
	{"CMake", ".cmake", "cmake", "IDM_LANG_CMAKE", "CMake", "#", ""},
	{"Fortran (free form)", ".f90", "fortran", "IDM_LANG_FORTRAN", "Fortran", "!", ""},
	{"Visual Basic", ".vb", "vb", "IDM_LANG_VB", "VB.net", "'", ""},
	{"Objective-C", ".m", "objc", "IDM_LANG_OBJC", "Objective-C", "//", "{[("},
	{"Verilog", ".v", "verilog", "IDM_LANG_VERILOG", "Verilog", "//", ""},
	{"VHDL", ".vhd", "vhdl", "IDM_LANG_VHDL", "VHDL", "--", ""},
	{"Matlab", "lang:matlab", "matlab", "IDM_LANG_MATLAB", "Matlab", "%", ""},
	{"Nim", ".nim", "nim", "IDM_LANG_NIM", "Nim", "#", ""},
	{"TCL", ".tcl", "tcl", "IDM_LANG_TCL", "Tcl", "#", ""},
	{"PostScript", ".ps", "ps", "IDM_LANG_PS", "PostScript", "%", ""},
	{"Raku", ".raku", "raku", "IDM_LANG_RAKU", "Raku", "#", ""},
	{"Scheme", ".scm", "scheme", "IDM_LANG_SCHEME", "Scheme", ";", ""},
	{"CoffeeScript", ".coffee", "coffeescript", "IDM_LANG_COFFEESCRIPT", "CoffeeScript", "#", ""},
	{"GDScript", ".gd", "gdscript", "IDM_LANG_GDSCRIPT", "GDScript", "#", ""},
	{"Ada", ".ada", "ada", "IDM_LANG_ADA", "Ada", "--", ""},
	{"AutoIt", ".au3", "autoit", "IDM_LANG_AU3", "AutoIt", ";", ""},
	{"Batch", ".bat", "batch", "IDM_LANG_BATCH", "Batchfile", "REM", ""},
	{"Caml", ".ml", "caml", "IDM_LANG_CAML", "OCaml", "", ""},
	{"COBOL", ".cob", "cobol", "IDM_LANG_COBOL", "COBOL", "", ""},
	{"D", ".d", "d", "IDM_LANG_D", "D", "//", "{[("},
	{"Fortran (fixed form)", ".f77", "fortran77", "IDM_LANG_FORTRAN_77", "FortranFixed", "", ""},
	{"Forth", ".forth", "forth", "IDM_LANG_FORTH", "Forth", "\\", ""},
	{"NSIS", ".nsi", "nsis", "IDM_LANG_NSIS", "NSIS", ";", ""},
	{"Properties", ".properties", "props", "IDM_LANG_PROPS", "properties", "#", ""},
	{"Registry", ".reg", "registry", "IDM_LANG_REGISTRY", "reg", ";;", ""},
	{"SAS", ".sas", "sas", "IDM_LANG_SAS", "SAS", "", ""},
	{"Smalltalk", ".st", "smalltalk", "IDM_LANG_SMALLTALK", "Smalltalk", "", ""},
	{"ActionScript", ".as", "actionscript", "IDM_LANG_FLASH", "ActionScript", "//", "{[("},
	{"Assembly", ".asm", "asm", "IDM_LANG_ASM", "NASM", ";", ""},
	{"BlitzBasic", ".bb", "blitzbasic", "IDM_LANG_BLITZBASIC", "BlitzBasic", "", ""},
	{"LISP", ".lisp", "lisp", "IDM_LANG_LISP", "Common Lisp", ";", "("},
	{"Microsoft Transact-SQL", ".tsql", "mssql", "IDM_LANG_MSSQL", "Transact-SQL", "--", ""},
	{"TeX", "lang:tex", "tex", "IDM_LANG_TEX", "TeX", "%", ""},
	{"JSON5", ".json5", "json5", "IDM_LANG_JSON5", "JSON5", "//", "{[("},
	{"Intel HEX", ".hex", "ihex", "IDM_LANG_IHEX", "Intel HEX", "", ""},
	{"S-Record", ".srec", "srec", "IDM_LANG_SREC", "Motorola S-Record", "", ""},
	{"Tektronix extended HEX", ".tek", "tehex", "IDM_LANG_TEHEX", "Tektronix extended HEX", "", ""},
	{"Resource file", ".rc", "rc", "IDM_LANG_RC", "Windows Resource", "//", ""},
	{"Inno Setup", ".iss", "inno", "IDM_LANG_INNO", "Inno Setup", ";", ""},
	{"ASN.1", ".mib", "asn1", "IDM_LANG_ASN1", "ASN.1", "", ""},
	{"MMIXAL", ".mms", "mmixal", "IDM_LANG_MMIXAL", "MMIXAL", "%", ""},
	{"ErrorList", ".err", "errorlist", "IDM_LANG_ERRORLIST", "ErrorList", "#", ""},
	{"Spice", ".scp", "spice", "IDM_LANG_SPICE", "Spice", "*", ""},
	{"KIXtart", ".kix", "kix", "IDM_LANG_KIX", "KIXtart", ";", ""},
	{"CSound", ".orc", "csound", "IDM_LANG_CSOUND", "CSound", ";", ""},
	{"REBOL", ".reb", "rebol", "IDM_LANG_REBOL", "REBOL", ";", ""},
	{"AviSynth", ".avs", "avs", "IDM_LANG_AVS", "AviSynth", "#", ""},
}

func Languages() []Language { return append([]Language(nil), languages[:]...) }

func LanguageByLabel(label string) (Language, bool) {
	for _, spec := range languages {
		if spec.Label == label {
			return spec, true
		}
	}
	return Language{}, false
}

func LanguageByMode(mode string) (Language, bool) {
	mode = strings.ToLower(mode)
	switch mode {
	case ".yml":
		mode = ".yaml"
	case ".adb", ".ads":
		mode = ".ada"
	case ".cmd":
		mode = ".bat"
	case ".mli":
		mode = ".ml"
	case ".cpy":
		mode = ".cob"
	case ".nsh":
		mode = ".nsi"
	case ".jsonc":
		mode = ".json5"
	case ".mot":
		mode = ".srec"
	case ".out":
		mode = ".scp"
	case ".sco", ".csd":
		mode = ".orc"
	case ".r2", ".r3":
		mode = ".reb"
	case ".avsi":
		mode = ".avs"
	}
	for _, spec := range languages {
		if spec.Mode == mode {
			return spec, true
		}
	}
	return Language{}, false
}

func LanguageByNPP(name string) (Language, bool) {
	switch strings.ToLower(name) {
	case "js":
		name = "javascript"
	case "ts":
		name = "typescript"
	case "py":
		name = "python"
	case "md":
		name = "markdown"
	}
	for _, spec := range languages {
		if strings.EqualFold(spec.NPPName, name) {
			return spec, true
		}
	}
	return Language{}, false
}
