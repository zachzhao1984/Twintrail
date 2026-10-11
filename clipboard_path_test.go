package main

import (
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
