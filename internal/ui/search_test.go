package ui

import (
	"github.com/dendec/zimlite/internal/textinput"
	"github.com/veandco/go-sdl2/sdl"
	"testing"
)

func TestKeyboardCapturesGlobalShortcuts(t *testing.T) {
	app := &App{}
	app.running.Store(true)
	s := &searchState{app: app, visible: true, editing: true, pendingLanguage: -1, profile: textinput.Profile{Methods: []string{"direct"}}}
	app.search = s
	input := &InputController{app: app, globalKeys: map[sdl.Scancode]func(){sdl.SCANCODE_Q: func() { app.running.Store(false) }}}
	input.ProcessEvent(&sdl.KeyboardEvent{Type: sdl.KEYDOWN, Keysym: sdl.Keysym{Scancode: sdl.SCANCODE_Q}})
	if !app.running.Load() {
		t.Fatal("typing Q quit the reader")
	}
	var event sdl.TextInputEvent
	event.Type = sdl.TEXTINPUT
	copy(event.Text[:], "q")
	input.ProcessEvent(&event)
	if s.text != "q" {
		t.Fatalf("text input lost: %q", s.text)
	}
}
