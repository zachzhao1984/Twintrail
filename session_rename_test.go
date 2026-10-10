package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTwintrailSessionMigration(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, legacyApplicationName, "session.json")
	current := filepath.Join(root, "Twintrail", "session.json")
	a := fixture(t)
	a.fonts.Terminal = fontSetting{"Monaco", 19}
	saved := a.snapshotSession()
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(legacy, data, 0600); err != nil {
		t.Fatal(err)
	}
	migrated, err := readSessionData(current)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(migrated, data) {
		t.Fatal("legacy session was not preserved")
	}
	var restored session
	if err = json.Unmarshal(migrated, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Fonts.Terminal != a.fonts.Terminal || restored.Panes[0].Tabs[0].Path != a.current().path {
		t.Fatal("tabs or terminal font lost")
	}
	if err = os.MkdirAll(filepath.Dir(current), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(current, []byte(`{"Active":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	selected, err := readSessionData(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(selected) != `{"Active":1}` {
		t.Fatal("legacy configuration overrode Twintrail configuration")
	}
	// Corrupt new settings must not silently revive an outdated session.
	if err = os.WriteFile(current, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	selected, err = readSessionData(current)
	if err != nil || string(selected) != "invalid" {
		t.Fatal("corrupt current file incorrectly fell back to old settings")
	}
}
