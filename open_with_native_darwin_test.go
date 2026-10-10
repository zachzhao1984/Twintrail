package main

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeApplicationDiscoveryAndIcon(t *testing.T) {
	path := t.TempDir() + "/example.txt"
	if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	applications, err := systemOpenWithApplications([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(applications) == 0 || !applications[0].Default {
		t.Fatal("default application missing")
	}
	t.Logf("native application discovery: %s, %d entries", time.Since(start), len(applications))
	start = time.Now()
	bitmap, err := systemApplicationIcon("/System/Applications/TextEdit.app")
	if err != nil {
		t.Fatal(err)
	}
	width, height := bitmap.Size()
	if width != 64 || height != 64 {
		t.Fatalf("icon size: %dx%d", width, height)
	}
	t.Logf("native 64px icon: %s", time.Since(start))
	drain := applicationPool()
	defer drain()
	source, err := filepath.Abs("resources/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	nsImage := applicationSend(applicationSend(objc.ID(objc.GetClass("NSImage")), "alloc"), "initWithContentsOfFile:", applicationString(source))
	defer applicationSend(nsImage, "release")
	bitmap, err = applicationBitmap(nsImage)
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(16).Background(ui.Hex("#222a32")).Children(func() { ui.Image(c, bitmap).Width(64).Height(64) })
	}, 96, 96)
	file, err := os.Create("/tmp/twintrail-native-application-icon.png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err = png.Encode(file, tt.Image()); err != nil {
		t.Fatal(err)
	}

}
