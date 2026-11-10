package runcommand

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/shlex"
)

const MaxTemplateBytes = 4096

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
	parts, err := splitTemplate(template)
	if err != nil {
		return Prepared{}, err
	}
	replacements, directory := variableReplacer(vars)
	for index := range parts {
		parts[index] = replacements.Replace(parts[index])
		if strings.IndexByte(parts[index], 0) >= 0 {
			return Prepared{}, fmt.Errorf("Run argument contains NUL")
		}
	}
	if parts[0] == "" {
		return Prepared{}, fmt.Errorf("Run executable is empty in this document")
	}
	return Prepared{Executable: parts[0], Args: parts[1:], Dir: directory}, nil
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
