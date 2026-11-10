package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type command struct {
	Menu string
	Path string
	ID   string
}

type coverageEntry struct {
	Status   string
	Action   string
	Evidence string
}

var (
	popupPattern = regexp.MustCompile(`^POPUP\s+"((?:[^"]|"")*)"`)
	itemPattern  = regexp.MustCompile(`^MENUITEM\s+"((?:[^"]|"")*)"\s*,\s*([A-Z][A-Z0-9_]*)`)
)

func cleanLabel(value string) string {
	value = strings.ReplaceAll(value, `""`, `"`)
	runes := []rune(value)
	var result []rune
	for index := 0; index < len(runes); index++ {
		if runes[index] == '&' {
			if index+1 < len(runes) && runes[index+1] == '&' {
				result = append(result, '&')
				index++
			}
			continue
		}
		result = append(result, runes[index])
	}
	return string(result)
}

func parseMainMenu(source io.Reader) ([]command, error) {
	scanner := bufio.NewScanner(source)
	var commands []command
	var stack []string
	inside, started, pending := false, false, ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !inside {
			inside = line == "IDR_M30_MENU MENU"
			continue
		}
		if !started {
			if line != "BEGIN" {
				return nil, fmt.Errorf("main menu is missing BEGIN")
			}
			started = true
			continue
		}
		switch {
		case line == "BEGIN":
			if pending == "" {
				return nil, fmt.Errorf("menu BEGIN has no popup")
			}
			stack = append(stack, pending)
			pending = ""
		case line == "END":
			if pending != "" {
				return nil, fmt.Errorf("popup %q has no BEGIN", pending)
			}
			if len(stack) == 0 {
				return commands, nil
			}
			stack = stack[:len(stack)-1]
		case strings.HasPrefix(line, "POPUP "):
			match := popupPattern.FindStringSubmatch(line)
			if pending != "" || match == nil {
				return nil, fmt.Errorf("invalid popup %q", line)
			}
			pending = cleanLabel(match[1])
		case strings.HasPrefix(line, "MENUITEM "):
			if line == "MENUITEM SEPARATOR" {
				continue
			}
			match := itemPattern.FindStringSubmatch(line)
			if pending != "" || match == nil {
				return nil, fmt.Errorf("invalid menu item %q", line)
			}
			if len(stack) == 0 {
				if !strings.HasSuffix(line, ", HELP") {
					return nil, fmt.Errorf("unexpected root menu item %q", line)
				}
				continue
			}
			path := append(append([]string(nil), stack...), cleanLabel(match[1]))
			commands = append(commands, command{Menu: stack[0], Path: strings.Join(path, " / "), ID: match[2]})
		case line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#"):
		default:
			return nil, fmt.Errorf("unknown menu syntax %q", line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("main menu is missing its closing END")
}

func parseCoverage(source io.Reader, commands []command) (map[string]coverageEntry, error) {
	known := make(map[string]bool, len(commands))
	for _, item := range commands {
		known[item.ID] = true
	}
	entries := make(map[string]coverageEntry)
	scanner := bufio.NewScanner(source)
	if !scanner.Scan() || scanner.Text() != "id\tstatus\taction\tevidence" {
		return nil, fmt.Errorf("coverage header must be id, status, action, evidence")
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 4 || !known[fields[0]] {
			return nil, fmt.Errorf("invalid or unknown coverage row %q", line)
		}
		entry := coverageEntry{Status: fields[1], Action: fields[2], Evidence: fields[3]}
		if entry.Status != "Local" && entry.Status != "Partial" && entry.Status != "Missing" ||
			entry.Status != "Missing" && (entry.Action == "" || entry.Evidence == "") {
			return nil, fmt.Errorf("unsubstantiated coverage row %q", line)
		}
		if _, exists := entries[fields[0]]; exists {
			return nil, fmt.Errorf("duplicate coverage ID %q", fields[0])
		}
		entries[fields[0]] = entry
	}
	return entries, scanner.Err()
}

func markdownCell(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

func renderCatalog(commands []command, coverage map[string]coverageEntry, source string) string {
	return renderCatalogForMenus(commands, coverage, source, []string{"Edit", "Search"})
}

func selectMenus(commands []command, selection string) ([]string, error) {
	if selection == "" {
		return nil, fmt.Errorf("menu selection is empty")
	}
	ordered := make([]string, 0)
	known := make(map[string]bool)
	for _, item := range commands {
		if !known[item.Menu] {
			known[item.Menu] = true
			ordered = append(ordered, item.Menu)
		}
	}
	if selection == "all" {
		return ordered, nil
	}
	wanted := make(map[string]bool)
	for _, part := range strings.Split(selection, ",") {
		name := strings.TrimSpace(part)
		if !known[name] || wanted[name] {
			return nil, fmt.Errorf("unknown or repeated menu %q", name)
		}
		wanted[name] = true
	}
	menus := make([]string, 0, len(wanted))
	for _, name := range ordered {
		if wanted[name] {
			menus = append(menus, name)
		}
	}
	return menus, nil
}

func renderCatalogForMenus(commands []command, coverage map[string]coverageEntry, source string, menus []string) string {
	var output strings.Builder
	title := "Notepad++ static menu command trace"
	if len(menus) == 2 && menus[0] == "Edit" && menus[1] == "Search" {
		title = "Notepad++ Edit and Search command trace"
	}
	fmt.Fprintf(&output, "# %s\n\nSource: %s. This lists static `IDR_M30_MENU` entries only; dynamic commands are separate. `Local` is a focused local test, not Windows or pixel acceptance. `Unreviewed` means no behavior claim has been established yet.\n\n", title, source)
	for _, menu := range menus {
		var rows []command
		for _, item := range commands {
			if item.Menu == menu {
				rows = append(rows, item)
			}
		}
		counts := map[string]int{}
		fmt.Fprintf(&output, "## %s (%d static commands)\n\n| Original path | Symbolic ID | Status | VivaPad action | Evidence |\n| --- | --- | --- | --- | --- |\n", menu, len(rows))
		for _, item := range rows {
			entry, ok := coverage[item.ID]
			if !ok {
				entry.Status = "Unreviewed"
			}
			counts[entry.Status]++
			evidence := ""
			if entry.Evidence != "" {
				evidence = "`" + markdownCell(entry.Evidence) + "`"
			}
			fmt.Fprintf(&output, "| %s | `%s` | %s | %s | %s |\n", markdownCell(item.Path), item.ID, entry.Status, markdownCell(entry.Action), evidence)
		}
		fmt.Fprintf(&output, "\n%s: %d local, %d partial, %d missing, %d unreviewed.\n\n", menu, counts["Local"], counts["Partial"], counts["Missing"], counts["Unreviewed"])
	}
	return output.String()
}

func main() {
	coveragePath := flag.String("coverage", "", "tab-separated coverage file")
	outputPath := flag.String("output", "", "Markdown report path")
	source := flag.String("source", "", "reference commit and file")
	menusFlag := flag.String("menus", "Edit,Search", "comma-separated source menus or all")
	flag.Parse()
	if *coveragePath == "" || *outputPath == "" || *source == "" {
		fmt.Fprintln(os.Stderr, "usage: nppcatalog -coverage FILE -output FILE -source REFERENCE [-menus Edit,Search|all] < Notepad_plus.rc")
		os.Exit(2)
	}
	commands, err := parseMainMenu(os.Stdin)
	if err == nil {
		var menus []string
		menus, err = selectMenus(commands, *menusFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var file *os.File
		file, err = os.Open(*coveragePath)
		if err == nil {
			defer file.Close()
			var entries map[string]coverageEntry
			entries, err = parseCoverage(file, commands)
			if err == nil {
				err = os.WriteFile(*outputPath, []byte(renderCatalogForMenus(commands, entries, *source, menus)), 0644)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
