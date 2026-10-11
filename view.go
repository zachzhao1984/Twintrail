package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"strings"
)

func (a *app) view(c *ui.Context) {
	width, _ := c.Size()
	if width > 0 && a.lastWidth > 0 && width != a.lastWidth {
		a.split *= width / a.lastWidth
	}
	if width > 0 {
		a.lastWidth = width
	}
	a.refreshTerminalMenu()
	theme := martaTheme(a.light)
	c.SetTheme(theme)
	ui.Column(c).Fill().Gap(0).Background(theme.Background).Children(func() {
		bar := c.TitleBar()
		ui.Row(c).Height(max(bar.Height, 28)).Background(theme.Background).Center().DragWindow().Children(func() {
			ui.Text(c, "Twintrail").TextAlign(ui.Center).FontSize(13).Bold().TextColor(theme.TextMuted)
		})
		ui.Split(c, &a.split, func() { a.paneView(c, 0) }, func() { a.paneView(c, 1) }).Grow(1)
		if a.status != "Ready" && a.status != "" {
			ui.Text(c, a.status).Height(20).PaddingX(6).TextColor(theme.TextMuted)
		}
		a.fonts.ActionBar.apply(ui.Row(c).Height(fontHeight(a.fonts.ActionBar, 20) + 6).Padding(3).Gap(3).Background(theme.Background)).Children(func() {
			for _, b := range []struct {
				name string
				fn   func()
			}{
				{"⌘↵ Open With", a.openWith}, {"F4 Edit", a.edit}, {"F5 Copy", func() { a.ask("Copy") }},
				{"F6 Move", func() { a.ask("Move") }}, {"⇧F6 Rename", func() { a.ask("Rename") }},
				{"F7 New Folder", func() { a.ask("New folder") }}, {"⇧F8 Delete", func() { a.ask("Delete") }},
			} {
				ui.ButtonBase(c).Grow(1).Height(fontHeight(a.fonts.ActionBar, 20)).Background(theme.Surface).Radius(2).PaddingX(5).AlignItems(ui.Center).Justify(ui.Start).Children(func() { ui.Text(c, b.name).TextColor(theme.TextMuted) }).OnClick(b.fn).Disabled(a.busy || a.modal != nil)
			}
		})
		if a.current().terminal.visible {
			a.terminalView(c)
		}
		if a.modal != nil {
			a.dialogView(c)
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyComma, a.openPreferences)
	c.OnShortcut(ui.Cmd, ui.KeyBackquote, a.toggleTerminal)
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyP, func() { a.ask("Actions") })
	if a.modal != nil || !a.current().terminal.handle.FocusWithin(c) {
		c.OnShortcut(0, ui.KeyEscape, func() {
			a.modal = nil
			a.current().filter = ""
			a.current().searchOpen = false
			a.current().marked = map[string]bool{}
			a.focus = a.active + 1
		})
	}
	if a.modal == nil && !a.current().terminal.handle.FocusWithin(c) {
		services := c.Services()
		c.OnShortcut(ui.Cmd, ui.KeyEnter, a.openWith)
		c.OnShortcut(0, ui.KeyF4, a.edit)
		c.OnShortcut(ui.Cmd, ui.KeyF, func() { a.current().searchOpen = true })
		c.OnShortcut(0, ui.KeySlash, func() { a.current().searchOpen = true })
		c.OnShortcut(0, ui.KeyF12, func() { services.WriteClipboard(a.current().path) })
		c.OnShortcut(ui.Cmd, ui.KeyF12, func() { services.WriteClipboard(strings.Join(a.targets(), "\n")) })
		c.OnShortcut(0, ui.KeyF11, func() { a.launch("open", "-a", "Terminal", a.current().path) })
		c.OnShortcut(0, ui.KeyTab, func() { a.active = 1 - a.active; a.focus = a.active + 1 })
		c.OnShortcut(ui.Cmd, ui.KeyT, func() {
			p := &a.panes[a.active]
			t := &tab{path: a.current().path, marked: map[string]bool{}}
			p.tabs = append(p.tabs, t)
			p.current = len(p.tabs) - 1
			a.load(t, t.path)
		})
		c.OnShortcut(ui.Cmd, ui.KeyW, func() {
			p := &a.panes[a.active]
			if len(p.tabs) > 1 {
				a.closeTerminal(p.tabs[p.current])
				p.tabs[p.current].generation++
				p.tabs = append(p.tabs[:p.current], p.tabs[p.current+1:]...)
				if p.current >= len(p.tabs) {
					p.current--
				}
			}
		})
		c.OnShortcut(ui.Cmd, ui.KeyBackquote, a.toggleTerminal)
		c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyPeriod, func() { t := a.current(); t.hidden = !t.hidden; a.load(t, t.path) })
		c.OnShortcut(ui.Cmd, ui.KeyR, func() { t := a.current(); a.load(t, t.path) })
		for _, s := range []struct {
			key  ui.Key
			mods ui.Modifiers
			kind string
		}{{ui.KeyF5, 0, "Copy"}, {ui.KeyF6, 0, "Move"}, {ui.KeyF6, ui.Shift, "Rename"}, {ui.KeyF7, 0, "New folder"}, {ui.KeyF8, 0, "Move to Trash"}, {ui.KeyF8, ui.Shift, "Delete"}, {ui.KeyL, ui.Cmd, "Go to path"}} {
			c.OnShortcut(s.mods, s.key, func() { a.ask(s.kind) })
		}
	}
}
func (a *app) paneView(c *ui.Context, index int) {
	p := &a.panes[index]
	t := p.tabs[p.current]
	theme := c.Theme()
	active := a.active == index
	c.Key(fmt.Sprintf("pane-%d", index))
	ui.Column(c).Fill().Gap(0).Disabled(a.modal != nil).Children(func() {
		if len(p.tabs) > 1 {
			ui.Row(c).Height(max(fontHeight(a.fonts.Tabs, 24), fontHeight(a.fonts.VirtualTabs, 24))).Gap(1).Background(theme.Border).Border(1, theme.Border).BorderWidth(0, 0, 1, 0).Children(func() {
				for i, tab := range p.tabs {
					bg := ui.Hex("#1e252c")
					foreground := ui.Hex("#787f87")
					if i == p.current && active {
						bg = ui.Hex("#252d36")
						foreground = ui.Hex("#aab1b9")
					}
					if a.light {
						bg = theme.Surface
						foreground = theme.TextMuted
						if i == p.current && active {
							bg = theme.Background
							foreground = theme.Text
						}
					}
					c.Key(fmt.Sprintf("tab-%p", tab))
					tabFont := a.fonts.Tabs
					if i == p.current {
						tabFont = a.fonts.VirtualTabs
					}
					tabFont.apply(ui.ButtonBase(c).Grow(1).Height(max(fontHeight(a.fonts.Tabs, 24), fontHeight(a.fonts.VirtualTabs, 24))).Background(bg).PaddingX(10)).Children(func() {
						name := filepath.Base(tab.path)
						if tab.path == "/" {
							name = "Macintosh HD"
						}
						ui.Text(c, name).SingleLine().TextColor(foreground)
					}).OnClick(func() { p.current = i; a.active = index; a.focus = index + 1 })
				}
			})
		}
		a.fonts.Breadcrumbs.apply(ui.Row(c).Height(fontHeight(a.fonts.Breadcrumbs, 24)).PaddingX(6).Gap(4).AlignItems(ui.Center)).Children(func() {
			ui.ButtonBase(c).Label(fmt.Sprintf("Root %d", index)).Children(func() { ui.Text(c, "/").TextColor(theme.TextMuted) }).OnClick(func() { a.active = index; a.load(t, "/") })
			parts := strings.Split(strings.Trim(t.path, "/"), "/")
			start := 0
			if len(parts) > 3 {
				start = len(parts) - 3
				ui.Button(c, "…").OnClick(func() { a.active = index; a.ask("Go to path") })
			}
			for i, part := range parts {
				if i < start {
					continue
				}
				if part == "" {
					continue
				}
				path := "/" + filepath.Join(parts[:i+1]...)
				ui.Text(c, "›").TextColor(theme.TextMuted)
				ui.ButtonBase(c).PaddingX(3).Children(func() { ui.Text(c, part).TextColor(theme.TextMuted) }).OnClick(func() { a.active = index; a.load(t, path) })
			}
		})
		header := headerBackground
		if a.light {
			header = theme.Surface
		}
		sizeWidth := max(float32(80), float32(a.fonts.Files.Size)*6.7)
		modifiedWidth := max(float32(136), float32(a.fonts.Files.Size)*11.5)
		rowHeight := fontHeight(a.fonts.Files, fileRowHeight)
		a.fonts.TableHeader.apply(ui.Row(c).Height(fontHeight(a.fonts.TableHeader, 20)).Background(header).Border(1, theme.Border).BorderWidth(1, 0, 1, 0).AlignItems(ui.Stretch)).Children(func() {
			ui.ButtonBase(c).Grow(1).PaddingX(4).Justify(ui.Start).Children(func() {
				ui.Row(c).Fill().AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Name").Grow(1).TextColor(theme.TextMuted)
					arrow := "⌃"
					if t.descending {
						arrow = "⌄"
					}
					ui.Text(c, arrow).TextColor(theme.TextMuted)
				})
			}).OnClick(func() { t.descending = !t.descending; sortEntries(t.entries, t.descending) })
			ui.Text(c, "Size").Width(sizeWidth).PaddingX(4).Border(1, theme.Border).BorderWidth(0, 0, 0, 1).TextColor(theme.TextMuted)
			ui.Text(c, "Modified").Width(modifiedWidth).PaddingX(4).Border(1, theme.Border).BorderWidth(0, 0, 0, 1).TextColor(theme.TextMuted)
		})
		es := t.visible()
		t.list.Selected = &t.cursor
		t.list.Label = func(i int) string { return es[i].name }
		c.Key(fmt.Sprintf("files-%d-%d", index, p.current))
		ui.List(c, &t.list, len(es)).Grow(1).FocusRing(false).Draw(func(p *ui.Painter, r ui.Rect) {
			alternate := rowAlternate
			if a.light {
				alternate = theme.Surface
			}
			for y := float32(0); y < r.H; y += rowHeight * 2 {
				p.Fill(ui.Rect{X: r.X, Y: r.Y + y + rowHeight, W: r.W, H: min(rowHeight, max(0, r.H-y-rowHeight))}, alternate, 0)
			}
		}).FocusBind(&a.focus, index+1).ItemKey(func(i int) any { return es[i].name }).Rows(func(r ui.ListRow) {
			e := es[r.Index]
			bg := theme.Background
			if r.Index%2 == 1 {
				bg = rowAlternate
				if a.light {
					bg = theme.Surface
				}
			}
			if r.Selected() && active {
				bg = theme.Accent
			}
			color := theme.Text
			if t.marked[e.name] {
				color = ui.Hex("#e6bc75")
			}
			a.fonts.Files.apply(ui.Row(c).Height(rowHeight).Background(bg).AlignItems(ui.Center)).Children(func() {
				ui.Row(c).Grow(1).PaddingX(4).Gap(4).AlignItems(ui.Center).Children(func() { fileIcon(c, e); ui.Text(c, e.name).TextColor(color).Grow(1).MinWidth(0).SingleLine() })
				size := martaSize(e.size)
				if e.dir {
					size = "--"
				}
				if e.name == ".." {
					size = ""
				}
				ui.Text(c, size).Width(sizeWidth).PaddingX(4).TextAlign(ui.End).TextColor(theme.TextMuted)
				date := modifiedText(e.modified)
				if e.name == ".." {
					date = ""
				}
				ui.Text(c, date).Width(modifiedWidth).PaddingX(4).TextColor(theme.TextMuted).SingleLine()
			}).ContextMenu(func(m *ui.Menu) { a.fileMenu(c, m, index, r.Index) })
		}).OnClick(func() { a.active = index; a.focus = index + 1 }).OnChange(func() { a.active = index }).OnSubmit(func() { a.active = index; a.open() }).OnShortcut(0, ui.KeyBackspace, func() { a.active = index; a.load(t, filepath.Dir(t.path)) }).OnShortcut(0, ui.KeySpace, func() {
			a.active = index
			targets := a.targets()
			if len(targets) > 0 {
				a.launch("qlmanage", append([]string{"-p"}, targets...)...)
			}
		}).OnShortcut(0, ui.KeyInsert, func() {
			if t.cursor >= 0 && t.cursor < len(es) && es[t.cursor].name != ".." {
				e := es[t.cursor]
				t.marked[e.name] = !t.marked[e.name]
			}
		}).OnShortcut(ui.Cmd, ui.KeyA, func() {
			for _, e := range t.entries {
				if e.name != ".." {
					t.marked[e.name] = true
				}
			}
		}).OnShortcut(ui.Shift, ui.KeyDown, func() { a.markMove(t, es, 1) }).OnShortcut(ui.Shift, ui.KeyUp, func() { a.markMove(t, es, -1) })
		if t.searchOpen {
			c.Key(fmt.Sprintf("search-%d", index))
			ui.TextInput(c, &t.filter).Height(24).Placeholder("Quick search").Label(fmt.Sprintf("Quick search %d", index)).Focus().OnChange(func() { a.active = index; t.cursor = 0 })
		}
		footer := footerBackground
		if a.light {
			footer = theme.Surface
		}
		a.fonts.StatusBar.apply(ui.Row(c).Height(fontHeight(a.fonts.StatusBar, 22)).PaddingX(4).Gap(4).AlignItems(ui.Center).Background(footer).Border(1, theme.Border).BorderWidth(1, 0, 0, 0)).Children(func() {
			ui.Text(c, tabSummary(t)).Grow(1).MinWidth(0).SingleLine().TextColor(theme.TextMuted)
			if t.hidden {
				ui.Text(c, "Hidden").TextColor(theme.TextMuted)
			}
		})
	})
}

// fileMenu fills a file row's context menu. The row that owns the menu is the
// one that was clicked, so the cursor moves to it and every item acts on that
// entry rather than on the row the cursor happened to be on. Marked entries
// still win, as targets returns them in place of the cursor's.
func (a *app) fileMenu(c *ui.Context, m *ui.Menu, index, row int) {
	a.active, a.focus = index, index+1
	t := a.panes[index].tabs[a.panes[index].current]
	if row >= 0 && row < len(t.visible()) {
		t.cursor = row
	}
	none := len(a.targets()) == 0 // the parent row, which has nothing to act on
	for _, kind := range []string{"Copy", "Move", "Rename", "New folder", "Move to Trash", "Delete"} {
		if m.Item(kind).Disabled(a.busy || none).Chosen() {
			a.ask(kind)
		}
	}
	m.Separator()
	if m.Item("Reveal in Finder").Disabled(none).Chosen() {
		if paths := a.targets(); len(paths) > 0 {
			a.launch("open", "-R", paths[0])
		}
	}
	if m.Item("Copy Path").Disabled(a.busy || none).Chosen() {
		if paths := a.targets(); len(paths) > 0 {
			c.Services().WriteClipboard(joinedPaths(paths))
		}
	}
	if m.Item("Open Terminal Here").Chosen() {
		a.launch("open", "-a", "Terminal", t.path)
	}
}
func (a *app) markMove(t *tab, es []entry, delta int) {
	if t.cursor >= 0 && t.cursor < len(es) && es[t.cursor].name != ".." {
		e := es[t.cursor]
		t.marked[e.name] = !t.marked[e.name]
	}
	t.cursor += delta
	if t.cursor < 0 {
		t.cursor = 0
	}
	if t.cursor >= len(es) {
		t.cursor = len(es) - 1
	}
	t.list.ScrollIntoView(t.cursor)
}
func (a *app) dialogView(c *ui.Context) {
	d := a.modal
	if !d.open {
		a.modal = nil
		a.focus = a.active + 1
		return
	}
	c.Key(fmt.Sprintf("dialog-%p", d))
	ui.DialogBase(c, &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.Hex("#00000055"))
		panelWidth := float32(440)
		if d.kind == "Preference" {
			panelWidth = 660
		}
		if d.kind == "Open With" {
			panelWidth = 360
		}
		panel.Width(panelWidth).Background(c.Theme().Background).Border(1, c.Theme().Border).Radius(7).Padding(16)
		ui.Column(c).Gap(10).Children(func() {
			if d.kind == "Open With" {
				a.openWithView(c, d)
				return
			}
			if d.kind == "Preference" {
				a.preferencesView(c)
				return
			}
			ui.Text(c, d.kind).Bold().FontSize(15)
			if d.kind == "Actions" {
				ui.TextInput(c, &d.value).Placeholder("Search actions…").Bind(&d.input).Focus()
				for _, kind := range []string{"Go to path", "Copy", "Move", "Rename", "New folder", "Move to Trash", "Delete"} {
					if strings.Contains(strings.ToLower(kind), strings.ToLower(d.value)) {
						ui.Button(c, kind).OnClick(func() { a.ask(kind) })
					}
				}
			} else {
				if len(d.sources) > 0 {
					ui.Text(c, fmt.Sprintf("%d item(s) · %s", len(d.sources), filepath.Base(d.sources[0]))).TextColor(c.Theme().TextMuted)
				}
				if d.kind != "Move to Trash" && d.kind != "Delete" {
					ui.TextInput(c, &d.value).Label(d.kind + " destination").Bind(&d.input).Focus().OnSubmit(a.confirm)
				} else {
					message := "Move the selected items to the macOS Trash?"
					if d.kind == "Delete" {
						message = "Permanently delete these items? This cannot be undone."
					}
					ui.Text(c, message)
				}
				ui.Row(c).Gap(8).Children(func() {
					ui.PrimaryButton(c, d.kind).Disabled(a.busy).OnClick(a.confirm)
					ui.Button(c, "Cancel").OnClick(func() { a.modal = nil; a.focus = a.active + 1 })
				})
			}
		})
	})
}
