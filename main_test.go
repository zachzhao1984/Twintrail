package main

import (
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) *app {
	t.Helper()
	left := t.TempDir()
	right := t.TempDir()
	if err := os.WriteFile(filepath.Join(left, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(left, "Folder"), 0755)
	os.WriteFile(filepath.Join(left, ".hidden"), nil, 0644)
	a := newApp(left, right)
	for i := range a.panes {
		tab := a.panes[i].tabs[0]
		a.load(tab, tab.path)
	}
	return a
}
func TestNavigationAndView(t *testing.T) {
	a := fixture(t)
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("hello.txt") {
		t.Fatal(tt.Texts())
	}
	if tt.HasText(".hidden") {
		t.Fatal("hidden file visible")
	}
	t1 := a.current()
	t1.hidden = true
	a.load(t1, t1.path)
	tt.Frame()
	if !tt.HasText(".hidden") {
		t.Fatal("hidden file missing")
	}
	if err := tt.Click("F7 New Folder"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.modal.value = "Created"
	a.confirm()
	tt.Frame()
	if !tt.HasText("Created") {
		t.Fatal("new folder missing")
	}
	for i := 0; i < 8; i++ {
		a.ask("Actions")
		tt.Frame()
		a.modal = nil
		tt.Frame()
	}
}
func TestTransfer(t *testing.T) {
	a := fixture(t)
	src := filepath.Join(a.current().path, "hello.txt")
	right := a.panes[1].tabs[0].path
	if err := perform(&dialog{kind: "Copy", sources: []string{src}, value: right}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(right, "hello.txt"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("copy %s %v", data, err)
	}
	if err := perform(&dialog{kind: "Copy", sources: []string{src}, value: right}); err == nil {
		t.Fatal("overwrite allowed")
	}
	if err := validateDestination(a.current().path, filepath.Join(a.current().path, "Folder", "nested")); err == nil {
		t.Fatal("recursive destination allowed")
	}
	if err := perform(&dialog{kind: "Rename", sources: []string{src}, target: a.current().path, value: "renamed.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := perform(&dialog{kind: "Move", sources: []string{filepath.Join(a.current().path, "renamed.txt")}, value: right}); err != nil {
		t.Fatal(err)
	}
}
func TestRender(t *testing.T) {
	a := fixture(t)
	img := ui.Render(a.view, 1100, 720, 1)
	f, err := os.Create("/tmp/twintrail-preview.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestKeyboard(t *testing.T) {
	a := fixture(t)
	tt := ui.NewTester(a.view, 1100, 720)
	tt.Key(0, ui.KeyTab)
	if a.active != 1 {
		t.Fatalf("Tab active=%d", a.active)
	}
	tt.Key(0, ui.KeyTab)
	if a.active != 0 {
		t.Fatalf("Tab active=%d", a.active)
	}
	tt.Key(0, ui.KeyDown)
	if a.current().cursor != 1 {
		t.Fatalf("Down cursor=%d", a.current().cursor)
	}
	tt.Key(ui.Shift, ui.KeyDown)
	if !a.current().marked["Folder"] {
		t.Fatal("Shift Down did not mark Folder")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	if !a.current().marked["hello.txt"] {
		t.Fatal("select all failed")
	}
	tt.Key(0, ui.KeyEscape)
	if len(a.current().marked) != 0 {
		t.Fatal("Escape did not clear selection")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	if a.modal == nil || a.modal.kind != "Actions" {
		t.Fatal("action panel missing")
	}
	tt.Type("rename")
	if a.modal.value != "rename" {
		t.Fatalf("action filter=%q", a.modal.value)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Key(0, ui.KeyF7)
	if a.modal == nil || a.modal.kind != "New folder" {
		t.Fatal("new folder dialog missing")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Cmd, ui.KeyT)
	if len(a.panes[0].tabs) != 2 {
		t.Fatal("new tab missing")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	if len(a.panes[0].tabs) != 1 {
		t.Fatal("tab close failed")
	}
}
