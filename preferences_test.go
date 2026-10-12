package main

import (
	"encoding/json"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreferencesLive(t *testing.T) {
	a := fixture(t)
	saved := make(chan session, 32)
	a.settingsWriter.save = func(s session) { saved <- s }
	tt := ui.NewTester(a.view, 1100, 720)
	before, ok := tt.Find("hello.txt")
	if !ok {
		t.Fatal("file text missing")
	}
	tt.Key(ui.Cmd, ui.KeyComma)
	if a.modal == nil || a.modal.kind != "Preference" {
		t.Fatal("preference shortcut failed")
	}
	if err := tt.Click("files size"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("22")
	tt.Frame()
	if a.fonts.Files.Size != 22 {
		t.Fatalf("font size=%v", a.fonts.Files.Size)
	}
	select {
	case s := <-saved:
		if s.Fonts.Files.Size != 22 {
			t.Fatal("font snapshot wrong")
		}
	case <-time.After(time.Second):
		t.Fatal("font change not persisted")
	}
	if err := tt.Click("breadcrumbs font"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Monaco")
	tt.Frame()
	if a.fonts.Breadcrumbs.Name != "Monaco" {
		t.Fatal("font family not applied")
	}
	f, e := os.Create("/tmp/twintrail-font-preferences-preview.png")
	if e != nil {
		t.Fatal(e)
	}
	if e = png.Encode(f, tt.Image()); e != nil {
		t.Fatal(e)
	}
	f.Close()
	if err := tt.Click("完成"); err != nil {
		t.Fatal(err)
	}
	after, ok := tt.Find("hello.txt")
	if !ok || after.H <= before.H {
		t.Fatalf("font not reflected in layout: before=%v after=%v", before, after)
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)
	if err := tt.Click("恢复默认字体"); err != nil {
		t.Fatalf("%v modal=%v texts=%q", err, a.modal, tt.Texts())
	}
	if a.fonts != defaultFonts() {
		t.Fatal("reset failed")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
}
func TestFontSettingsRoundTrip(t *testing.T) {
	a := fixture(t)
	a.fonts.Files = fontSetting{"Monaco", 19}
	data, e := json.Marshal(a.snapshotSession())
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "session.json")
	if e = os.WriteFile(p, data, 0600); e != nil {
		t.Fatal(e)
	}
	data, e = os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	var s session
	if e = json.Unmarshal(data, &s); e != nil {
		t.Fatal(e)
	}
	if s.Fonts.normalized().Files != a.fonts.Files {
		t.Fatal("font preferences lost")
	}
	if (fontSettings{}).normalized() != defaultFonts() {
		t.Fatal("legacy session defaults wrong")
	}
	m := a.applicationMenu()
	if it := m.ItemByID("preferences"); it == nil || it.Accelerator != "Cmd+," {
		t.Fatal("Preference menu missing")
	}
}

// The real updater.MenuItem reads the user's language, which asks the app's
// event loop, and a test has none. The stub keeps the app's menu shape the
// same while giving the test a build that returns.
func init() {
	updateMenuItem = func() *mygo.MenuItem {
		return &mygo.MenuItem{Label: "Check for Updates…", Click: func(*mygo.MenuItem, *mygo.Window) {}}
	}
}

// TestAppMenuCarriesUpdateItem checks that the app menu carries the
// updater's "Check for Updates…" item, which the plugin's docs place right
// after About.
//
// The item is built through the updateMenuItem seam, not updater.MenuItem
// itself, because building the real one reads the user's language, which
// blocks on an app event loop a test does not run.
func TestAppMenuCarriesUpdateItem(t *testing.T) {
	a := fixture(t)
	m := a.applicationMenu()
	found := false
	for _, top := range m.Items() {
		for _, it := range top.Submenu {
			if strings.Contains(it.Label, "Check for Updates") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("the app menu has no Check for Updates item")
	}
}

func TestTerminalFontPreferencesLive(t *testing.T) {
	a := fixture(t)
	a.settingsWriter.save = func(session) {}
	visible := &fakeTerminal{}
	hidden := &fakeTerminal{}
	a.panes[0].tabs[0].terminal.session = visible
	a.panes[1].tabs[0].terminal.session = hidden
	tt := ui.NewTester(a.view, 1100, 760)
	tt.Key(ui.Cmd, ui.KeyComma)
	if err := tt.Click("terminal size"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("18")
	tt.Frame()
	if visible.font.Size != 18 || hidden.font.Size != 18 {
		t.Fatalf("terminal sizes: %v %v", visible.font, hidden.font)
	}
	if err := tt.Click("terminal font"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Monaco")
	tt.Frame()
	if visible.font.Name != "Monaco" || hidden.font.Name != "Monaco" {
		t.Fatal("font family not applied")
	}
	f, err := os.Create("/tmp/twintrail-terminal-font-preferences-preview.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := tt.Click("恢复默认字体"); err != nil {
		t.Fatal(err)
	}
	if visible.font != defaultFonts().Terminal || hidden.font != defaultFonts().Terminal {
		t.Fatal("reset not applied to terminals")
	}
}

func TestTerminalFontRestoredAndPendingSession(t *testing.T) {
	fonts := defaultFonts()
	fonts.Terminal = fontSetting{"Monaco", 20}
	data, err := json.Marshal(session{Fonts: fonts})
	if err != nil {
		t.Fatal(err)
	}
	var restored session
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Fonts.normalized().Terminal != fonts.Terminal {
		t.Fatal("terminal font not persisted")
	}
	legacy := fonts
	legacy.Terminal = fontSetting{}
	if legacy.normalized().Terminal != defaultFonts().Terminal {
		t.Fatal("legacy session missing terminal defaults")
	}
	a := fixture(t)
	completions := make(chan func(), 1)
	a.update = func(done func()) { completions <- done }
	created := &fakeTerminal{}
	a.newTerminal = func(string) (terminalSession, error) { return created, nil }
	a.ensureTerminal(a.current())
	a.fonts.Terminal = fonts.Terminal
	done := <-completions
	done()
	if created.font != fonts.Terminal {
		t.Fatal("new terminal did not receive latest font")
	}
}
