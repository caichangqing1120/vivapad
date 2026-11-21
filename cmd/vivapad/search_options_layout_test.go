package main

import (
	"testing"

	"github.com/vivaui/viva/headless"
)

func TestSearchRegexOptionContainsItsLinuxWidth(t *testing.T) {
	state := newModel()
	state.searchOpen = true
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		t.Run(size.name, func(t *testing.T) {
			if err := session.Resize(size.width, size.height); err != nil {
				t.Fatal(err)
			}
			snapshot := session.Snapshot()
			snapshot.TextWidths = append([]float32(nil), snapshot.TextWidths...)
			matched := 0
			for index, text := range snapshot.Texts {
				if text.Value == "Regular expression" {
					snapshot.TextWidths[index] = 105
					matched++
				}
			}
			if matched != 1 {
				t.Fatalf("search regex label count = %d", matched)
			}
			if problems := headless.CheckLayout(snapshot); len(problems) != 0 {
				t.Fatalf("Linux-width search label spills at %s: %+v", size.name, problems)
			}
		})
	}
}
