package main

import (
	"github.com/egoist/mygo/ui"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenWithSelection(t *testing.T) {
	a := fixture(t)
	a.current().cursor = 2 // hello.txt, after parent and Folder
	a.discoverApplications = func(paths []string) ([]openWithApplication, error) {
		if len(paths) != 1 {
			t.Fatalf("selection: %v", paths)
		}
		return []openWithApplication{{Name: "TextEdit", Path: "/System/Applications/TextEdit.app", Default: true}, {Name: "Visual Studio Code", Path: "/Applications/Visual Studio Code.app"}}, nil
	}
	var chosen string
	var sources []string
	a.openApplication = func(path string, paths []string) { chosen = path; sources = paths }
	tt := ui.NewTester(a.view, 1100, 720)
	a.openWith()
	tt.Frame()
	if !tt.HasText("TextEdit (default)") || !tt.HasText("All Applications…") {
		t.Fatal(tt.Texts())
	}
	f, err := os.Create("/tmp/twintrail-open-with-preview.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := tt.Click("Visual Studio Code"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if chosen != "/Applications/Visual Studio Code.app" || len(sources) != 1 || a.modal != nil {
		t.Fatalf("selection not dispatched: %q %v", chosen, sources)
	}
	a.openWith()
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.modal != nil {
		t.Fatal("Escape did not dismiss picker")
	}
}

func TestOpenWithDiscardClosedPicker(t *testing.T) {
	a := fixture(t)
	a.current().cursor = 2
	completions := make(chan func(), 1)
	a.update = func(done func()) { completions <- done }
	a.discoverApplications = func([]string) ([]openWithApplication, error) { return []openWithApplication{{Name: "TextEdit"}}, nil }
	a.openWith()
	previous := a.modal
	a.modal = nil
	done := <-completions
	done()
	if a.modal != nil || !previous.loading {
		t.Fatal("closed picker received a stale result")
	}
}

func TestOpenWithCachedList(t *testing.T) {
	a := fixture(t)
	a.current().cursor = 2
	calls := 0
	a.discoverApplications = func([]string) ([]openWithApplication, error) {
		calls++
		return []openWithApplication{{Name: "Default Application", Default: true}}, nil
	}
	a.openWith()
	a.modal = nil
	a.openWith()
	if calls != 1 {
		t.Fatalf("reopening repeated application discovery: %d calls", calls)
	}
	if a.modal.loading || len(a.modal.applications) != 1 {
		t.Fatal("cached list not immediately visible")
	}
}

func TestOpenWithListBeforeIcons(t *testing.T) {
	a := fixture(t)
	a.current().cursor = 2
	updates := make(chan func(), 8)
	a.update = func(done func()) { updates <- done }
	a.discoverApplications = func([]string) ([]openWithApplication, error) {
		return []openWithApplication{{Name: "TextEdit", Path: "/System/Applications/TextEdit.app", Default: true}}, nil
	}
	started := make(chan struct{})
	release := make(chan struct{})
	a.loadApplicationIcon = func(string) (*ui.Bitmap, error) {
		close(started)
		<-release
		return ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 64, 64))), nil
	}
	a.openWith()
	(<-updates)()
	<-started
	if a.modal.loading || len(a.modal.applications) != 1 || a.modal.applications[0].bitmap != nil {
		t.Fatal("application list waited for icon")
	}
	a.modal = nil
	a.openWith()
	if a.modal.loading || len(a.modal.applications) != 1 {
		t.Fatal("reopened list waited for ongoing icon job")
	}
	close(release)
	(<-updates)()
	if a.modal.applications[0].bitmap == nil {
		t.Fatal("icon not delivered to current picker")
	}
	a.modal = nil
	a.openWith()
	if a.modal.applications[0].bitmap == nil {
		t.Fatal("cached icon not immediately visible")
	}
}

func TestOpenWithCacheUsesSelectionAndRefreshes(t *testing.T) {
	a := fixture(t)
	a.current().cursor = 2
	calls := 0
	a.discoverApplications = func(paths []string) ([]openWithApplication, error) {
		calls++
		return []openWithApplication{{Name: filepath.Base(paths[0]), Default: true}}, nil
	}
	a.openWith()
	a.current().cursor = 1
	a.openWith()
	if calls != 2 || a.modal.applications[0].Name != "Folder" {
		t.Fatal("cached applications reused for another selection")
	}
	key := applicationSelectionKey(a.modal.sources)
	cached := a.applicationLists[key]
	cached.loaded = time.Now().Add(-2 * time.Minute)
	a.applicationLists[key] = cached
	updates := make(chan func(), 1)
	a.update = func(done func()) { updates <- done }
	a.discoverApplications = func([]string) ([]openWithApplication, error) {
		return []openWithApplication{{Name: "New Default", Default: true}}, nil
	}
	a.openWith()
	if a.modal.loading || a.modal.applications[0].Name != "Folder" {
		t.Fatal("stale cache was hidden during refresh")
	}
	(<-updates)()
	if a.modal.applications[0].Name != "New Default" {
		t.Fatal("changed association not refreshed")
	}
}

func BenchmarkOpenWithCachedList(b *testing.B) {
	a := newApp(b.TempDir(), b.TempDir())
	a.current().entries = []entry{{name: "example.txt"}}
	a.discoverApplications = func([]string) ([]openWithApplication, error) {
		return []openWithApplication{{Name: "Default Application", Default: true}}, nil
	}
	a.openWith()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.openWith()
	}
}
