package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestJoinedPaths(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths []string
		want  string
	}{
		{"empty", nil, ""},
		{"single", []string{"/Users/zach/Workspace/twinrail/README.md"}, "/Users/zach/Workspace/twinrail/README.md"},
		{"several", []string{"/a/one.txt", "/a/two.txt"}, "/a/one.txt\n/a/two.txt"},
		{"space", []string{"/a/My Documents/notes.md"}, "'/a/My Documents/notes.md'"},
		{"double quote", []string{`/a/say "hi".txt`}, `'/a/say "hi".txt'`},
		{"single quote", []string{"/a/o'brien.txt"}, `'/a/o'\''brien.txt'`},
		{"tab", []string{"/a/two\tcolumns.txt"}, "'/a/two\tcolumns.txt'"},
		{"newline", []string{"/a/two\nlines.txt"}, "'/a/two\nlines.txt'"},
		{"shell syntax", []string{"/a/$(rm -rf ~)/x.txt"}, "'/a/$(rm -rf ~)/x.txt'"},
		{"empty path", []string{""}, "''"},
		{"quoted lines join", []string{"/a/one file.txt", "/a/two.txt"}, "'/a/one file.txt'\n/a/two.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinedPaths(tc.paths); got != tc.want {
				t.Fatalf("joinedPaths(%q) = %q, want %q", tc.paths, got, tc.want)
			}
		})
	}
}

func TestCopyPathMenuItem(t *testing.T) {
	a := fixture(t)
	tt := ui.NewTester(a.view, 1100, 720)
	if err := tt.RightClick("hello.txt"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy Path"); err != nil {
		t.Fatalf("%v (menu: %v)", err, tt.Menu())
	}
	want := filepath.Join(a.current().path, "hello.txt")
	if got := tt.Clipboard(); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}

	// Marked entries copy as every path, one per line.
	a = fixture(t)
	tt = ui.NewTester(a.view, 1100, 720)
	dir := a.current().path
	if err := os.WriteFile(filepath.Join(dir, "a file.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	a.load(a.current(), dir)
	tt.Frame()
	a.current().marked["hello.txt"] = true
	a.current().marked["a file.txt"] = true
	if err := tt.RightClick("hello.txt"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy Path"); err != nil {
		t.Fatalf("%v (menu: %v)", err, tt.Menu())
	}
	// targets() lists marked entries in the order the list shows them.
	want = joinedPaths([]string{filepath.Join(dir, "a file.txt"), filepath.Join(dir, "hello.txt")})
	if got := tt.Clipboard(); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}
}

func TestCopyPathDisabledWhileBusy(t *testing.T) {
	a := fixture(t)
	tt := ui.NewTester(a.view, 1100, 720)
	a.busy = true
	a.current().cursor = 2
	tt.Frame()
	if err := tt.RightClick("hello.txt"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy Path"); err == nil {
		t.Fatal("Copy Path was choosable while busy")
	}
	if got := tt.Clipboard(); got != "" {
		t.Fatalf("clipboard = %q, want empty", got)
	}
}

// rowNames lists the entry names the file list shows, which start with the
// parent entry when the directory has a parent.
func rowNames(t *tab) []string {
	out := make([]string, 0, len(t.entries))
	for _, e := range t.entries {
		out = append(out, e.name)
	}
	return out
}

// TestContextMenuActsOnClickedRow checks that the row a context menu was
// opened on, not the row the cursor was on, is the one the items act on.
func TestContextMenuActsOnClickedRow(t *testing.T) {
	a := fixture(t)
	tt := ui.NewTester(a.view, 1100, 720)
	// The cursor starts on the parent row.
	a.current().cursor = 0
	tt.Frame()
	if names := rowNames(a.current()); len(names) == 0 || names[0] != ".." {
		t.Fatalf("fixture rows: %v", names)
	}
	if err := tt.RightClick("hello.txt"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy Path"); err != nil {
		t.Fatalf("%v (menu: %v)", err, tt.Menu())
	}
	if got, want := tt.Clipboard(), filepath.Join(a.current().path, "hello.txt"); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}
	if got := a.current().cursor; got != 2 {
		t.Fatalf("cursor = %d, want 2 (hello.txt)", got)
	}

	// The same holds between two real rows, where every item stays enabled:
	// the cursor sits on Folder, the menu opens on hello.txt.
	a = fixture(t)
	tt = ui.NewTester(a.view, 1100, 720)
	a.current().cursor = 1 // Folder
	tt.Frame()
	if err := tt.RightClick("hello.txt"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy Path"); err != nil {
		t.Fatalf("%v (menu: %v)", err, tt.Menu())
	}
	if got, want := tt.Clipboard(), filepath.Join(a.current().path, "hello.txt"); got != want {
		t.Fatalf("clipboard = %q, want %q (not the cursor's Folder)", got, want)
	}
	if got := a.current().cursor; got != 2 {
		t.Fatalf("cursor = %d, want 2 (hello.txt)", got)
	}
}

// TestContextMenuWithMarkedEntries checks that a marked set still wins over
// the row the menu was opened on.
func TestContextMenuWithMarkedEntries(t *testing.T) {
	a := fixture(t)
	tt := ui.NewTester(a.view, 1100, 720)
	dir := a.current().path
	if err := os.WriteFile(filepath.Join(dir, "a file.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	a.load(a.current(), dir)
	tt.Frame()
	// Mark hello.txt, then open the menu on a row that is not marked.
	a.current().marked["hello.txt"] = true
	a.current().cursor = 1
	tt.Frame()
	if err := tt.RightClick("Folder"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy Path"); err != nil {
		t.Fatalf("%v (menu: %v)", err, tt.Menu())
	}
	if got, want := tt.Clipboard(), filepath.Join(dir, "hello.txt"); got != want {
		t.Fatalf("clipboard = %q, want the marked entry %q", got, want)
	}
}

// TestContextMenuRenameActsOnClickedRow checks a dialog-opening item picks the
// clicked row, which is the strongest check that the cursor moved.
func TestContextMenuRenameActsOnClickedRow(t *testing.T) {
	a := fixture(t)
	tt := ui.NewTester(a.view, 1100, 720)
	a.current().cursor = 0 // the parent row
	tt.Frame()
	if err := tt.RightClick("Folder"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Rename"); err != nil {
		t.Fatalf("%v (menu: %v)", err, tt.Menu())
	}
	if a.modal == nil {
		t.Fatal("Rename opened no dialog")
	}
	if got, want := a.modal.value, "Folder"; got != want {
		t.Fatalf("Rename value = %q, want %q", got, want)
	}
	if len(a.modal.sources) != 1 || a.modal.sources[0] != filepath.Join(a.current().path, "Folder") {
		t.Fatalf("Rename sources = %v, want the clicked Folder", a.modal.sources)
	}
}

// TestContextMenuParentRow checks the parent row, which has nothing to act
// on, keeps a menu with its items disabled.
func TestContextMenuParentRow(t *testing.T) {
	a := fixture(t)
	tt := ui.NewTester(a.view, 1100, 720)
	if err := tt.RightClick(".."); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if len(menu) == 0 {
		t.Fatal("the parent row has no menu")
	}
	if err := tt.ChooseMenuItem("Copy Path"); err == nil {
		t.Fatalf("Copy Path was choosable on the parent row (menu %v)", menu)
	}
	// Open Terminal Here does not act on an entry, so it stays available.
	tt.CloseMenu()
	if err := tt.RightClick(".."); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Open Terminal Here"); err != nil {
		t.Fatalf("Open Terminal Here on the parent row: %v", err)
	}
}

// rowBackground returns the background colour the file list draws for the row
// showing name, at the centre of its row box.
func rowBackground(t *testing.T, img *image.RGBA, tt *ui.Tester, name string) string {
	t.Helper()
	r, ok := tt.Find(name)
	if !ok {
		t.Fatalf("no row shows %q", name)
	}
	i := img.PixOffset(int(r.X+r.W/2), int(r.Y+r.H/2))
	return fmt.Sprintf("%02x%02x%02x", img.Pix[i], img.Pix[i+1], img.Pix[i+2])
}

// TestContextMenuMovesHighlight checks the row the context menu opened on is
// drawn as the chosen row while the menu is up.
//
// The runtime shows the native menu once the frame it opened in is already
// presented, and the menu stays up until an item is chosen. A highlight drawn
// only in that frame is therefore never seen, so the list must be drawn again
// after the cursor moved. The headless host has no native menu and presents
// every frame it settles, so what this asserts is the redraw the fix asks for.
func TestContextMenuMovesHighlight(t *testing.T) {
	a := fixture(t)
	accent := martaTheme(false).Accent
	want := fmt.Sprintf("%02x%02x%02x", accent.R, accent.G, accent.B)
	presented := 0
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		presented++
	}, 1100, 720)
	a.current().cursor = 1 // Folder
	tt.Frame()
	if got := rowBackground(t, tt.Image(), tt, "hello.txt"); got == want {
		t.Fatalf("hello.txt already shows as chosen before any right-click: %s", got)
	}
	presented = 0
	if err := tt.RightClick("hello.txt"); err != nil {
		t.Fatal(err)
	}
	// Right-clicking a row the cursor is not on must ask for a frame after
	// the cursor moved: two frames open the menu, and the third presents the
	// highlight the menu would otherwise cover. Without the redraw only two
	// frames run and the row stays unhighlighted until the menu closes.
	if presented < 3 {
		t.Fatalf("the list was drawn %d time(s) for a right-click on another row, want a frame drawn again after the cursor moved", presented)
	}
	if got := rowBackground(t, tt.Image(), tt, "hello.txt"); got != want {
		t.Fatalf("hello.txt drawn %s with the menu up, want the chosen colour %s", got, want)
	}
	if got := rowBackground(t, tt.Image(), tt, "Folder"); got == want {
		t.Fatalf("Folder is still drawn as chosen: %s", got)
	}
}

// TestContextMenuAlreadyFocusedRow checks right-clicking the row the cursor is
// already on asks for no extra frame: nothing moved, so nothing to redraw.
func TestContextMenuAlreadyFocusedRow(t *testing.T) {
	a := fixture(t)
	presented := 0
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		presented++
	}, 1100, 720)
	a.current().cursor = 2 // hello.txt
	tt.Frame()
	presented = 0
	if err := tt.RightClick("hello.txt"); err != nil {
		t.Fatal(err)
	}
	if presented > 2 {
		t.Fatalf("the list was drawn %d time(s) for a right-click on the cursor's own row, want none beyond the menu's", presented)
	}
}
