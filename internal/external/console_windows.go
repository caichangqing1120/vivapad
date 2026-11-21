package external

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func OpenConsole(kind ConsoleKind, path string) error {
	dir, err := consoleDirectory(path)
	if err != nil {
		return err
	}
	volume := strings.ToUpper(filepath.VolumeName(dir))
	if kind == CommandPrompt && strings.HasPrefix(volume, `\\`) &&
		(!strings.HasPrefix(volume, `\\?\`) || strings.HasPrefix(volume, `\\?\UNC\`)) {
		return errors.New("Command Prompt cannot start in a UNC folder; use PowerShell or a mapped drive")
	}
	program, err := windowsConsoleProgram(kind)
	if err != nil {
		return err
	}
	process, err := startWindowsConsole(program, dir)
	if err != nil {
		return fmt.Errorf("could not open console: %w", err)
	}
	_ = windows.CloseHandle(process)
	return nil
}

func windowsConsoleProgram(kind ConsoleKind) (string, error) {
	var program string
	switch kind {
	case CommandPrompt:
		program = os.Getenv("COMSPEC")
		if program == "" {
			return "", errors.New("Command Prompt is not configured (COMSPEC is empty)")
		}
	case WindowsPowerShell:
		const keyPath = `SOFTWARE\Microsoft\PowerShell\1\ShellIds\Microsoft.PowerShell`
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
		if err != nil {
			return "", fmt.Errorf("Windows PowerShell is not registered: %w", err)
		}
		defer key.Close()
		var valueType uint32
		program, valueType, err = key.GetStringValue("Path")
		if err == nil && valueType == registry.EXPAND_SZ {
			program, err = registry.ExpandString(program)
		}
		if err != nil {
			return "", fmt.Errorf("Windows PowerShell executable is unavailable: %w", err)
		}
	default:
		return "", errors.New("unknown Windows console")
	}
	if !filepath.IsAbs(program) || !strings.EqualFold(filepath.Ext(program), ".exe") || strings.IndexByte(program, 0) >= 0 {
		return "", errors.New("configured console must be an absolute executable path")
	}
	return program, nil
}

func startWindowsConsole(program, dir string, args ...string) (windows.Handle, error) {
	image, err := windows.UTF16PtrFromString(program)
	if err != nil {
		return 0, err
	}
	workingDir, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	commandLine := windows.EscapeArg(program)
	for _, arg := range args {
		commandLine += " " + windows.EscapeArg(arg)
	}
	command, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		return 0, err
	}
	// Leave STARTF_USESTDHANDLES unset so Windows gives the child its new console handles.
	var startup windows.StartupInfo
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var process windows.ProcessInformation
	if err := windows.CreateProcess(image, command, nil, nil, false, windows.CREATE_NEW_CONSOLE,
		nil, workingDir, &startup, &process); err != nil {
		return 0, err
	}
	_ = windows.CloseHandle(process.Thread)
	return process.Process, nil
}
