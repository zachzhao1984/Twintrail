package main

import (
	"math"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type fontSetting struct {
	Name string
	Size float64
}
type fontSettings struct {
	ActionBar   fontSetting `json:"actionBar"`
	Breadcrumbs fontSetting `json:"breadcrumbs"`
	VirtualTabs fontSetting `json:"virtualTabs"`
	Files       fontSetting `json:"files"`
	StatusBar   fontSetting `json:"statusBar"`
	TableHeader fontSetting `json:"tableHeader"`
	Tabs        fontSetting `json:"tabs"`
	Preferences fontSetting `json:"preferences"`
	Terminal    fontSetting `json:"terminal"`
}

func defaultFonts() fontSettings {
	return fontSettings{fontSetting{"Default", 12}, fontSetting{"Default", 14}, fontSetting{"Default Bold", 12}, fontSetting{"Default", 14}, fontSetting{"Default Light", 11}, fontSetting{"Default", 11}, fontSetting{"Default", 12}, fontSetting{"Monaco", 14}, fontSetting{"Default", 12}}
}
func (f fontSetting) familyWeight() (string, int) {
	family := strings.TrimSpace(f.Name)
	weight := 400
	switch family {
	case "Default", "":
		family = ""
	case "Default Bold":
		family = ""
		weight = 700
	case "Default Light":
		family = ""
		weight = 300
	}
	return family, weight
}
func (f fontSetting) apply(e ui.Element) ui.Element {
	family, weight := f.familyWeight()
	return e.Font(family).FontWeight(weight).FontSize(float32(f.Size))
}
func (f *fontSettings) values() []*fontSetting {
	return []*fontSetting{&f.ActionBar, &f.Breadcrumbs, &f.VirtualTabs, &f.Files, &f.StatusBar, &f.TableHeader, &f.Tabs, &f.Preferences, &f.Terminal}
}
func (f fontSettings) normalized() fontSettings {
	defaults := defaultFonts()
	d := defaults.values()
	for i, v := range f.values() {
		if strings.TrimSpace(v.Name) == "" {
			v.Name = d[i].Name
		}
		if math.IsNaN(v.Size) || math.IsInf(v.Size, 0) || v.Size < 8 || v.Size > 48 {
			v.Size = d[i].Size
		}
	}
	return f
}
func fontHeight(f fontSetting, minimum float32) float32 {
	return max(minimum, float32(math.Ceil(f.Size*1.4+4)))
}
func (a *app) openPreferences() {
	a.focus = 0
	a.modal = &dialog{kind: "Preference", open: true}
}
func (a *app) preferenceMenu() *mygo.MenuItem {
	return &mygo.MenuItem{ID: "preferences", Label: "Preference…", Accelerator: "Cmd+,", Click: func(item *mygo.MenuItem, w *mygo.Window) {
		a.openPreferences()
		if w != nil {
			w.Invalidate()
		}
	}}
}

// Serialize writes and reject snapshots superseded by a more recent edit.
type sessionWriter struct {
	mu      sync.Mutex
	version atomic.Uint64
	save    func(session)
}

func (a *app) persistSettings(background bool) {
	snapshot := a.snapshotSession()
	revision := a.settingsWriter.version.Add(1)
	work := func() {
		a.settingsWriter.mu.Lock()
		defer a.settingsWriter.mu.Unlock()
		if revision == a.settingsWriter.version.Load() {
			save := a.settingsWriter.save
			if save == nil {
				save = saveSession
			}
			save(snapshot)
		}
	}
	if background {
		go work()
	} else {
		work()
	}
}
func (a *app) preferencesView(c *ui.Context) {
	_, h := c.Size()
	a.fonts.Preferences.apply(ui.Column(c).Gap(12)).Children(func() {
		ui.Text(c, "字体设置").FontSize(float32(a.fonts.Preferences.Size) + 4).Bold()
		ui.Text(c, "修改立即生效并自动保存。Default 使用系统字体，Terminal 使用系统等宽字体。").TextColor(c.Theme().TextMuted)
		ui.Scroll(c).MaxHeight(max(160, h-180)).Children(func() {
			a.fonts.Preferences.apply(ui.Column(c).Gap(8)).Children(func() {
				names := []string{"actionBar", "breadcrumbs", "virtualTabs", "files", "statusBar", "tableHeader", "tabs", "preferences", "terminal"}
				for i, setting := range a.fonts.values() {
					name := names[i]
					c.Key(name)
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, name).Width(132).SingleLine()
						ui.TextInput(c, &setting.Name).Width(224).Label(name + " font").OnChange(func() {
							if strings.TrimSpace(setting.Name) != "" {
								a.fontsChanged()
							}
						})
						ui.NumberInput(c, &setting.Size, 8, 48, 1).Width(112).Label(name + " size").OnChange(func() { a.fontsChanged() })
						ui.Text(c, "pt").FontSize(11).TextColor(c.Theme().TextMuted)
					})
				}
			})
		})
		ui.Row(c).Gap(10).Children(func() {
			ui.Button(c, "恢复默认字体").OnClick(func() { a.fonts = defaultFonts(); a.fontsChanged() })
			ui.PrimaryButton(c, "完成").OnClick(func() { a.modal = nil; a.focus = a.active + 1 })
		})
	})
}

// Apply to existing sessions, including terminals hidden in other tabs.
func (a *app) fontsChanged() {
	for _, pane := range a.panes {
		for _, tab := range pane.tabs {
			if tab.terminal.session != nil {
				tab.terminal.session.SetFont(a.fonts.Terminal)
			}
		}
	}
	a.persistSettings(true)
}
