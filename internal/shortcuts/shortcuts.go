package shortcuts

import (
	"fmt"
	"maps"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Modifiers uint8

const (
	Primary Modifiers = 1 << iota
	Alt
	Shift
	Control
)

type Chord struct {
	Modifiers Modifiers `json:"modifiers,omitempty"`
	Key       string    `json:"key,omitempty"`
}

func (binding Chord) Empty() bool { return binding == (Chord{}) }

var functionKeys = [...]string{"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12"}

func (binding Chord) Valid() bool {
	if binding.Empty() {
		return true
	}
	if binding.Modifiers&^(Primary|Alt|Shift|Control) != 0 || binding.Modifiers&(Primary|Control) == Primary|Control {
		return false
	}
	for _, key := range functionKeys {
		if binding.Key == key {
			return true
		}
	}
	switch binding.Key {
	case "Escape":
		return binding.Modifiers == 0
	case "Tab", "Enter", "Space", "Up", "Down":
		return binding.Modifiers&(Primary|Control|Alt) != 0
	}
	key, size := utf8.DecodeRuneInString(binding.Key)
	return size == len(binding.Key) && key != utf8.RuneError && unicode.IsGraphic(key) && !unicode.IsSpace(key) &&
		unicode.ToUpper(key) == key && binding.Modifiers&(Primary|Control|Alt) != 0
}

func (binding Chord) Format(platform string) string {
	if binding.Empty() {
		return ""
	}
	var labels []string
	if platform == "darwin" {
		for _, modifier := range []struct {
			flag Modifiers
			name string
		}{{Control, "⌃"}, {Alt, "⌥"}, {Shift, "⇧"}, {Primary, "⌘"}} {
			if binding.Modifiers&modifier.flag != 0 {
				labels = append(labels, modifier.name)
			}
		}
		return strings.Join(append(labels, binding.Key), "")
	}
	if binding.Modifiers&(Primary|Control) != 0 {
		labels = append(labels, "Ctrl")
	}
	if binding.Modifiers&Alt != 0 {
		labels = append(labels, "Alt")
	}
	if binding.Modifiers&Shift != 0 {
		labels = append(labels, "Shift")
	}
	return strings.Join(append(labels, binding.Key), "+")
}

type Command struct {
	ID              string
	Name            string
	Default         Chord
	Aliases         []Chord
	Fixed           bool
	OptionalDefault bool
}

type ConflictError struct{ Command string }

func (err *ConflictError) Error() string {
	return fmt.Sprintf("Shortcut already assigned to %s", err.Command)
}

type Registry struct {
	path      string
	commands  map[string]Command
	overrides map[string]Chord
	keys      map[Chord]string
}

func (registry *Registry) Binding(id string) Chord {
	if binding, ok := registry.overrides[id]; ok {
		return binding
	}
	command := registry.commands[id]
	if command.OptionalDefault && registry.Lookup(command.Default) != id {
		return Chord{}
	}
	return command.Default
}

func (registry *Registry) Lookup(binding Chord) string { return registry.keys[binding] }

func (registry *Registry) Register(catalog []Command) error {
	commands := maps.Clone(registry.commands)
	for _, command := range catalog {
		if command.Name == "" {
			command.Name = command.ID
		}
		commands[command.ID] = command
	}
	candidate := *registry
	candidate.commands = commands
	keys, err := candidate.index(registry.overrides)
	if err == nil {
		registry.commands, registry.keys = commands, keys
	}
	return err
}

func (registry *Registry) Conflict(id string, binding Chord) error {
	if !binding.Valid() {
		return fmt.Errorf("Unsupported shortcut")
	}
	if binding.Empty() {
		return nil
	}
	for _, key := range physicalVariants(binding) {
		if other := registry.Lookup(key); other != "" && other != id {
			return &ConflictError{registry.commands[other].Name}
		}
	}
	return nil
}

func physicalVariants(binding Chord) []Chord {
	other := binding
	if binding.Modifiers&(Primary|Control) != 0 {
		other.Modifiers ^= Primary | Control
		return []Chord{binding, other}
	}
	return []Chord{binding}
}

func (registry *Registry) index(overrides map[string]Chord) (map[Chord]string, error) {
	keys := make(map[Chord]string)
	for id, command := range registry.commands {
		bindings := append([]Chord{command.Default}, command.Aliases...)
		if binding, ok := overrides[id]; ok {
			if command.Fixed {
				return nil, fmt.Errorf("%s is handled by native text input", command.Name)
			}
			bindings = []Chord{binding}
		} else if command.OptionalDefault {
			continue
		}
		for _, binding := range bindings {
			if binding.Empty() {
				continue
			}
			for _, key := range physicalVariants(binding) {
				if other := keys[key]; other != "" && other != id {
					return nil, &ConflictError{registry.commands[other].Name}
				}
			}
			keys[binding] = id
		}
	}
	for id, command := range registry.commands {
		if _, overridden := overrides[id]; !overridden && command.OptionalDefault && !command.Default.Empty() {
			available := true
			for _, key := range physicalVariants(command.Default) {
				available = available && keys[key] == ""
			}
			if available {
				keys[command.Default] = id
			}
		}
	}
	return keys, nil
}
