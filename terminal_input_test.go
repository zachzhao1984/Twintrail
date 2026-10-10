package main

import (
	"github.com/egoist/mygo/ui"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNativeTerminalKeyboardInput(t *testing.T) {
	if os.Getenv("MYGO_GHOSTTY_VT") == "" {
		t.Skip("native runtime required")
	}
	t.Setenv("ZDOTDIR", t.TempDir())
	a := fixture(t)
	session, err := newTerminal(a.current().path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	native := session.(*nativeTerminal)
	a.current().terminal.visible = true
	a.current().terminal.session = session
	a.focus = 3
	focused := false
	var bounds ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		focused = a.current().terminal.handle.Focused(c)
		bounds = a.current().terminal.handle.Bounds(c)
	}, 1100, 720)
	deadline := time.Now().Add(3 * time.Second)
	for native.term.Dir() == "" && time.Now().Before(deadline) {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	tt.Frame()
	tt.Frame()
	if !focused {
		t.Fatalf("terminal not focused: desired=%d bounds=%v", a.focus, bounds)
	}

	tt.Type("echo __UI_INPUT_WORKS__")
	tt.Key(0, ui.KeyEnter)
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if strings.Count(native.term.Text(), "__UI_INPUT_WORKS__") >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Count(native.term.Text(), "__UI_INPUT_WORKS__") < 2 {
		t.Fatalf("typed command did not execute; focus=%d output=%q", a.focus, native.term.Text())
	}
	// Changing the selected file used to change the terminal parent's key.
	a.current().cursor = 1
	tt.Frame()
	tt.Frame()
	if !focused {
		t.Fatal("file selection stole terminal focus")
	}
	a.focus = 1
	tt.Frame()
	tt.Frame()
	tt.ClickAt(bounds.X+20, bounds.Y+30)
	tt.Frame()
	if !focused {
		t.Fatal("clicking terminal did not restore focus")
	}
	tt.Type("echo __UI_CLICK_WORKS__")
	tt.Key(0, ui.KeyEnter)
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if strings.Count(native.term.Text(), "__UI_CLICK_WORKS__") >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("command after refocus did not execute: %q", native.term.Text())
}
