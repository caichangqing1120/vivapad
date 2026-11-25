package runcommand

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/shlex"
)

const MaxTemplateBytes = 4096

var windowsEnvironmentReference = regexp.MustCompile(`%[\pL\pN_][\pL\pN_.()-]*%`)

type Variables struct {
	Path, Word, LineText, AppPath string
	Line, Column                  int
}

type Prepared struct {
	Executable string
	Args       []string
	Dir        string
}

func splitTemplate(template string) ([]string, error) {
	if strings.TrimSpace(template) == "" || len(template) > MaxTemplateBytes || !utf8.ValidString(template) || strings.IndexFunc(template, unicode.IsControl) >= 0 {
		return nil, fmt.Errorf("invalid Run command")
	}
	parts, err := shlex.Split(template)
	if err != nil || len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("invalid quoted Run command: %v", err)
	}
	return parts, nil
}

func variableReplacer(vars Variables) (*strings.Replacer, string) {
	directory, fileName, namePart, extPart := "", "", "", ""
	if vars.Path != "" {
		directory = filepath.Dir(vars.Path)
		fileName = filepath.Base(vars.Path)
		extPart = filepath.Ext(fileName)
		namePart = strings.TrimSuffix(fileName, extPart)
	}
	appDirectory := ""
	if vars.AppPath != "" {
		appDirectory = filepath.Dir(vars.AppPath)
	}
	return strings.NewReplacer(
		"$(FULL_CURRENT_PATH)", vars.Path,
		"$(CURRENT_DIRECTORY)", directory,
		"$(FILE_NAME)", fileName,
		"$(NAME_PART)", namePart,
		"$(EXT_PART)", extPart,
		"$(CURRENT_WORD)", vars.Word,
		"$(NPP_DIRECTORY)", appDirectory,
		"$(NPP_FULL_FILE_PATH)", vars.AppPath,
		"$(CURRENT_LINE)", strconv.Itoa(vars.Line),
		"$(CURRENT_COLUMN)", strconv.Itoa(vars.Column),
		"$(CURRENT_LINESTR)", vars.LineText,
	), directory
}

func ExpandVariables(value string, vars Variables) string {
	replacements, _ := variableReplacer(vars)
	return replacements.Replace(value)
}

func Prepare(template string, vars Variables) (Prepared, error) {
	if runtime.GOOS == "windows" {
		return prepare(template, vars, os.LookupEnv)
	}
	return prepare(template, vars, nil)
}

func prepare(template string, vars Variables, lookupEnv func(string) (string, bool)) (Prepared, error) {
	parts, err := splitTemplate(template)
	if err != nil {
		return Prepared{}, err
	}
	replacements, directory := variableReplacer(vars)
	for index := range parts {
		parts[index], err = expandRunArgument(parts[index], replacements, lookupEnv)
		if err != nil {
			return Prepared{}, err
		}
		if strings.IndexByte(parts[index], 0) >= 0 {
			return Prepared{}, fmt.Errorf("Run argument contains NUL")
		}
	}
	if parts[0] == "" {
		return Prepared{}, fmt.Errorf("Run executable is empty in this document")
	}
	return Prepared{Executable: parts[0], Args: parts[1:], Dir: directory}, nil
}

func expandRunArgument(argument string, vars *strings.Replacer, lookupEnv func(string) (string, bool)) (string, error) {
	if lookupEnv == nil {
		return vars.Replace(argument), nil
	}
	references := windowsEnvironmentReference.FindAllStringIndex(argument, -1)
	if len(references) == 0 {
		return vars.Replace(argument), nil
	}
	var expanded strings.Builder
	previous := 0
	for _, reference := range references {
		start, end := reference[0], reference[1]
		name := argument[start+1 : end-1]
		value, ok := lookupEnv(name)
		if !ok {
			return "", fmt.Errorf("unknown Run environment variable %%%s%%", name)
		}
		if !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return "", fmt.Errorf("invalid value for Run environment variable %%%s%%", name)
		}
		// Only template text is eligible for context expansion; inserted values are literal.
		expanded.WriteString(vars.Replace(argument[previous:start]))
		expanded.WriteString(value)
		previous = end
	}
	expanded.WriteString(vars.Replace(argument[previous:]))
	return expanded.String(), nil
}

func Launch(prepared Prepared) error {
	command := exec.Command(prepared.Executable, prepared.Args...)
	command.Dir = prepared.Dir
	if err := command.Start(); err != nil {
		return err
	}
	go func() { _ = command.Wait() }()
	return nil
}
