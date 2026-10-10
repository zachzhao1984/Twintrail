package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/egoist/mygo/ui"
)

const fileRowHeight float32 = 20

var rowAlternate = ui.Hex("#252d36")
var headerBackground = ui.Hex("#252d36")
var footerBackground = ui.Hex("#20272f")
var folderIcons = loadSystemIcons()

func martaTheme(light bool) *ui.Theme {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#222a32")
	t.Surface = ui.Hex("#2d3742")
	t.SurfaceHover = ui.Hex("#354352")
	t.SurfacePressed = ui.Hex("#293b52")
	t.Border = ui.Hex("#343e48")
	t.Text = ui.Hex("#d6dae0")
	t.TextMuted = ui.Hex("#9fa7af")
	t.Accent = ui.Hex("#254674")
	t.AccentText = t.Text
	t.Focus = ui.Transparent
	t.FontSize = 12
	t.Spacing = 3
	t.Radius = 2
	if light {
		t = *ui.LightTheme()
		t.FontSize = 12
		t.Spacing = 3
		t.Radius = 2
	}
	return &t
}

// Read the small Retina PNG representations from macOS' own icon files.
// Icons are decoded once, outside view construction, and aren't bundled.
func loadSystemIcons() map[string]*ui.Bitmap {
	out := map[string]*ui.Bitmap{}
	for name, file := range map[string]string{"folder": "GenericFolderIcon", "file": "GenericDocumentIcon", "Applications": "ApplicationsFolderIcon", "Desktop": "DesktopFolderIcon", "Developer": "DeveloperFolderIcon", "Documents": "DocumentsFolderIcon", "Downloads": "DownloadsFolder", "Movies": "MovieFolderIcon", "Music": "MusicFolderIcon", "Pictures": "PicturesFolderIcon", "Public": "PublicFolderIcon"} {
		data, err := os.ReadFile("/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/" + file + ".icns")
		if err != nil || len(data) < 8 || string(data[:4]) != "icns" {
			continue
		}
		var smallest *ui.Bitmap
		for at := 8; at+8 <= len(data); {
			size := int(binary.BigEndian.Uint32(data[at+4 : at+8]))
			if size < 8 || at+size > len(data) {
				break
			}
			chunk := data[at+8 : at+size]
			if bytes.HasPrefix(chunk, []byte("\x89PNG\r\n\x1a\n")) {
				bitmap, e := ui.DecodeBitmap(chunk)
				if e == nil {
					w, _ := bitmap.Size()
					old := 100000
					if smallest != nil {
						old, _ = smallest.Size()
					}
					if w >= 32 && w < old {
						smallest = bitmap
					}
				}
			}
			at += size
		}
		if smallest != nil {
			out[name] = smallest
		}
	}
	return out
}
func fileIcon(c *ui.Context, e entry) {
	key := "file"
	if e.dir {
		key = "folder"
		if _, ok := folderIcons[e.name]; ok {
			key = e.name
		}
	}
	if icon := folderIcons[key]; icon != nil {
		ui.Image(c, icon).Width(16).Height(16).Fit(ui.Contain).Shrink(0)
	} else {
		ui.Box(c).Width(16).Height(16).Draw(func(p *ui.Painter, r ui.Rect) {
			color := ui.Hex("#dce5ed")
			if e.dir {
				color = ui.Hex("#45bbdf")
			}
			p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 3, W: 14, H: 12}, color, 1)
		})
	}
}
func modifiedText(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "Today at " + t.Format("15:04")
	}
	yesterday := now.AddDate(0, 0, -1)
	if t.Year() == yesterday.Year() && t.YearDay() == yesterday.YearDay() {
		return "Yesterday at " + t.Format("15:04")
	}
	return t.Format("2 Jan 2006 at 15:04")
}
func martaSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d bytes", n)
	}
	for _, unit := range []struct {
		n    int64
		name string
	}{{1 << 30, "GB"}, {1 << 20, "MB"}, {1 << 10, "KB"}} {
		if n >= unit.n {
			return fmt.Sprintf("%.0f %s", float64(n)/float64(unit.n), unit.name)
		}
	}
	return ""
}
func tabSummary(t *tab) string {
	files, folders := 0, 0
	var total int64
	for _, e := range t.entries {
		if e.name == ".." {
			continue
		}
		if e.dir {
			folders++
		} else {
			files++
			total += e.size
		}
	}
	text := fmt.Sprintf("%d folders", folders)
	if files > 0 {
		text = fmt.Sprintf("%s in %d files, also %d folders", martaSize(total), files, folders)
	}
	selected := 0
	for _, v := range t.marked {
		if v {
			selected++
		}
	}
	if selected > 0 {
		text += fmt.Sprintf(" · %d selected", selected)
	}
	if t.disk != "" {
		text += "  |  " + t.disk
	}
	if t.loading {
		return "Loading…"
	}
	return text
}
func (a *app) edit() {
	paths := a.targets()
	if len(paths) > 0 {
		a.launch("open", append([]string{"-a", "TextEdit"}, paths...)...)
	}
}
