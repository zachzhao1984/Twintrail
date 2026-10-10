package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type fakeTerminal struct {
	path   string
	closed bool
	font   fontSetting
}

func (f *fakeTerminal) View(c *ui.Context) ui.Element             { return ui.Box(c).Focusable().Label("Terminal") }
func (f *fakeTerminal) SetDir(path string, revision uint64) error { f.path = path; return nil }
func (f *fakeTerminal) SetFont(font fontSetting)                  { f.font = font }
func (f *fakeTerminal) Close() error                              { f.closed = true; return nil }
func TestTabTerminal(t *testing.T) {
	a := fixture(t)
	created := []*fakeTerminal{}
	a.newTerminal = func(path string) (terminalSession, error) {
		f := &fakeTerminal{path: path}
		created = append(created, f)
		return f, nil
	}
	tt := ui.NewTester(a.view, 1100, 720)
	tt.Key(ui.Cmd, ui.KeyBackquote)
	left := a.current()
	if !left.terminal.visible || len(created) != 1 || created[0].path != left.path || !slices.Contains(tt.Texts(), "Terminal") {
		t.Fatal("terminal did not open in selected tab")
	}
	next := filepath.Join(left.path, "Folder")
	a.load(left, next)
	tt.Frame()
	if created[0].path != next {
		t.Fatal("terminal CWD did not follow navigation")
	}
	a.load(left, filepath.Join(next, "missing"))
	tt.Frame()
	if created[0].path != next {
		t.Fatal("failed navigation changed CWD")
	}
	a.toggleTerminal()
	tt.Frame()
	if slices.Contains(tt.Texts(), "Terminal") || created[0].closed {
		t.Fatalf("hiding should preserve shell: active=%d left=%v right=%v closed=%v texts=%q", a.active, left.terminal.visible, a.current().terminal.visible, created[0].closed, tt.Texts())
	}
	a.toggleTerminal()
	tt.Frame()
	if len(created) != 1 {
		t.Fatal("toggle restarted shell")
	}
	a.active = 1
	a.focus = 2
	tt.Frame()
	if slices.Contains(tt.Texts(), "Terminal") {
		t.Fatal("terminal leaked into other tab")
	}
	a.toggleTerminal()
	tt.Frame()
	if len(created) != 2 || created[1].path != a.current().path {
		t.Fatal("other tab did not get independent shell")
	}
	a.active = 0
	a.focus = 1
	tt.Frame()
	if !slices.Contains(tt.Texts(), "Terminal") {
		t.Fatal("original terminal was not restored")
	}
	a.closed = true
	a.closeTerminals()
	if !created[0].closed || !created[1].closed {
		t.Fatal("sessions not closed")
	}
	tt.Frame()
}
func TestTerminalStartError(t *testing.T) {
	a := fixture(t)
	a.newTerminal = func(string) (terminalSession, error) { return nil, fmt.Errorf("terminal library unavailable") }
	tt := ui.NewTester(a.view, 1100, 720)
	a.toggleTerminal()
	tt.Frame()
	if !tt.HasText("terminal library unavailable") {
		t.Fatal(tt.Texts())
	}
	a.toggleTerminal()
	tt.Frame()
	a.toggleTerminal()
	tt.Frame()
}
func TestTerminalMenu(t *testing.T) {
	a := fixture(t)
	m := a.applicationMenu()
	it := m.ItemByID("toggle-terminal")
	if it == nil || it.Label != "Toggle Terminal" {
		t.Fatal("View menu missing terminal action")
	}
	found := false
	for _, parent := range m.Items() {
		if parent.Label == "View" {
			for _, child := range parent.Submenu {
				if child == it {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("terminal item not under View")
	}
}

// Requires the official runtime; optional under ordinary unit-test runs.
// Exercise the actual shell while preserving partially typed input and
// while a foreground program is running, including quoted directory names.
func TestNativeTerminalCWD(t *testing.T) {
	if os.Getenv("MYGO_GHOSTTY_VT") == "" {
		t.Skip("set MYGO_GHOSTTY_VT for native integration")
	}
	t.Setenv("ZDOTDIR", t.TempDir())
	first := t.TempDir()
	second := filepath.Join(t.TempDir(), "directory ' $() with spaces")
	if err := os.Mkdir(second, 0755); err != nil {
		t.Fatal(err)
	}
	session, err := newTerminal(first)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	native := session.(*nativeTerminal)
	a := newApp(first, second)
	for i := range a.panes {
		tab := a.panes[i].tabs[0]
		a.load(tab, tab.path)
	}
	a.current().terminal.visible = true
	a.current().terminal.session = native
	a.focus = 3
	tt := ui.NewTester(a.view, 1100, 720)
	wait := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			tt.Frame()
			if check() {
				return
			}
			time.Sleep(15 * time.Millisecond)
		}
		t.Fatalf("terminal condition timed out: %q; CWD %q", native.term.Text(), native.term.Dir())
	}
	wait(func() bool { return native.term.Dir() == first })
	native.term.Send([]byte("printf '__PRESERVED__'"))
	if err = session.SetDir(second, 1); err != nil {
		t.Fatal(err)
	}
	wait(func() bool { return native.term.Dir() == second })
	native.term.Send([]byte("\r"))
	wait(func() bool { return strings.Count(native.term.Text(), "__PRESERVED__") >= 2 })
	native.term.Send([]byte("sleep 0.5; printf '__FINISHED__'\r"))
	time.Sleep(80 * time.Millisecond)
	if err = session.SetDir(first, 2); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	if native.term.Dir() != second {
		t.Fatal("changed foreground process CWD")
	}
	wait(func() bool {
		return native.term.Dir() == first && strings.Count(native.term.Text(), "__FINISHED__") >= 2
	})
	if err = session.SetDir(second, 1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if native.term.Dir() != first {
		t.Fatal("stale revision applied")
	}
	colsBefore, _ := native.term.Size()
	session.SetFont(fontSetting{"Monaco", 18})
	wait(func() bool { cols, _ := native.term.Size(); return cols > 0 && cols < colsBefore })
	tt.Frame()
	imageFile, err := os.Create("/tmp/twintrail-terminal-preview.png")
	if err != nil {
		t.Fatal(err)
	}
	defer imageFile.Close()
	if err = png.Encode(imageFile, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalCreatedAfterWindowClose(t *testing.T) {
	a := fixture(t)
	updates := make(chan func(), 8)
	started := make(chan struct{})
	release := make(chan struct{})
	session := &fakeTerminal{}
	a.update = func(fn func()) { updates <- fn }
	a.newTerminal = func(path string) (terminalSession, error) { close(started); <-release; return session, nil }
	a.toggleTerminal()
	<-started
	a.closed = true
	a.closeTerminals()
	close(release)
	a.finishTerminals()
	if !session.closed {
		t.Fatal("unpublished terminal leaked at shutdown")
	}
	select {
	case done := <-updates:
		done()
		a.finishTerminals()
	case <-time.After(time.Second):
		t.Fatal("startup completion missing")
	}
}
