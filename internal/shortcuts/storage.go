package shortcuts

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
)

const storageVersion = 1

type storedBindings struct {
	Version  int              `json:"version"`
	Bindings map[string]Chord `json:"bindings"`
}

func Load(path string, catalog []Command) (*Registry, error) {
	registry := &Registry{path: path, commands: make(map[string]Command, len(catalog))}
	for _, command := range catalog {
		if command.Name == "" {
			command.Name = command.ID
		}
		registry.commands[command.ID] = command
	}
	var err error
	registry.keys, err = registry.index(nil)
	if err != nil || path == "" {
		return registry, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return registry, nil
	}
	if err != nil {
		return registry, err
	}
	var stored storedBindings
	if err := json.Unmarshal(data, &stored); err != nil {
		return registry, err
	}
	if stored.Version != storageVersion {
		return registry, fmt.Errorf("unsupported shortcuts version %d", stored.Version)
	}
	for id, binding := range stored.Bindings {
		if id == "" || !binding.Valid() {
			return registry, fmt.Errorf("invalid binding for %q", id)
		}
	}
	keys, err := registry.index(stored.Bindings)
	if err == nil {
		registry.overrides, registry.keys = stored.Bindings, keys
	}
	return registry, err
}

func (registry *Registry) Set(id string, binding Chord) error {
	if _, ok := registry.commands[id]; !ok {
		return fmt.Errorf("unknown shortcut command %q", id)
	}
	if err := registry.Conflict(id, binding); err != nil {
		return err
	}
	candidate := make(map[string]Chord, len(registry.overrides)+1)
	maps.Copy(candidate, registry.overrides)
	candidate[id] = binding
	return registry.commit(candidate)
}

func (registry *Registry) Reset(id string) error {
	command, ok := registry.commands[id]
	if !ok {
		return fmt.Errorf("unknown shortcut command %q", id)
	}
	if err := registry.Conflict(id, command.Default); err != nil {
		return err
	}
	for _, alias := range command.Aliases {
		if err := registry.Conflict(id, alias); err != nil {
			return err
		}
	}
	candidate := maps.Clone(registry.overrides)
	delete(candidate, id)
	return registry.commit(candidate)
}

func (registry *Registry) ResetAll() error { return registry.commit(nil) }

func (registry *Registry) commit(candidate map[string]Chord) error {
	keys, err := registry.index(candidate)
	if err != nil {
		return err
	}
	if registry.path != "" {
		if err := save(registry.path, candidate); err != nil {
			return err
		}
	}
	registry.overrides, registry.keys = candidate, keys
	return nil
}

func save(path string, bindings map[string]Chord) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".shortcuts-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(storedBindings{storageVersion, bindings}); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return replaceFile(file.Name(), path)
}
