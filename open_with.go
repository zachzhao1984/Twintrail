package main

import (
	"encoding/json"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"time"
)

type openWithApplication struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Default bool   `json:"default"`
	bitmap  *ui.Bitmap
}

type cachedApplications struct {
	applications []openWithApplication
	loaded       time.Time
}

func applicationSelectionKey(paths []string) string {
	// Exact paths preserve per-file default handlers and package associations.
	key, _ := json.Marshal(paths)
	return string(key)
}
func (a *app) openWith() {
	paths := a.targets()
	if len(paths) == 0 {
		return
	}
	d := &dialog{open: true, kind: "Open With", sources: paths, loading: true}
	a.modal = d
	a.focus = 0
	if a.applicationLists == nil {
		a.applicationLists = make(map[string]cachedApplications)
		a.applicationJobs = make(map[string]bool)
		a.applicationIcons = make(map[string]*ui.Bitmap)
		a.applicationIconJobs = make(map[string]bool)
	}
	key := applicationSelectionKey(paths)
	cached, exists := a.applicationLists[key]
	if exists {
		d.loading = false
		a.showApplications(d, cached.applications)
	}
	if exists && time.Since(cached.loaded) < time.Minute {
		return
	}
	if a.applicationJobs[key] {
		return
	}
	a.applicationJobs[key] = true
	discover := a.discoverApplications
	if discover == nil {
		discover = systemOpenWithApplications
	}
	a.job(func() func() {
		applications, err := discover(paths)
		return func() {
			if a.closed {
				return
			}
			delete(a.applicationJobs, key)
			if err == nil {
				if len(a.applicationLists) >= 64 {
					clear(a.applicationLists)
				}
				a.applicationLists[key] = cachedApplications{applications: applications, loaded: time.Now()}
			}
			current := a.modal
			if current == nil || current.kind != "Open With" || applicationSelectionKey(current.sources) != key {
				return
			}
			current.loading = false
			if err != nil {
				current.err = err.Error()
				return
			}
			a.showApplications(current, applications)
		}
	})
}
func (a *app) showApplications(d *dialog, applications []openWithApplication) {
	d.applications = append([]openWithApplication(nil), applications...)
	for i := range d.applications {
		application := &d.applications[i]
		if icon, exists := a.applicationIcons[application.Path]; exists {
			application.bitmap = icon
		}
	}
	// The names are published first. Icon work cannot delay selecting an app.
	for _, application := range d.applications {
		path := application.Path
		if path == "" {
			continue
		}
		if _, exists := a.applicationIcons[path]; exists || a.applicationIconJobs[path] {
			continue
		}
		a.applicationIconJobs[path] = true
		load := a.loadApplicationIcon
		if load == nil {
			load = systemApplicationIcon
		}
		a.job(func() func() {
			icon, err := load(path)
			return func() {
				if a.closed {
					return
				}
				delete(a.applicationIconJobs, path)
				if err != nil {
					return
				}
				a.applicationIcons[path] = icon
				if current := a.modal; current != nil && current.kind == "Open With" {
					for i := range current.applications {
						if current.applications[i].Path == path {
							current.applications[i].bitmap = icon
						}
					}
				}
			}
		})
	}
}

func (a *app) chooseApplication(d *dialog, path string) {
	if a.modal != d {
		return
	}
	a.modal = nil
	a.focus = a.active + 1
	if a.openApplication != nil {
		a.openApplication(path, d.sources)
		return
	}
	args := append([]string(nil), d.sources...)
	if path != "" {
		args = append([]string{"-a", path}, args...)
	}
	a.launch("open", args...)
}

func (a *app) chooseOtherApplication(d *dialog) {
	if d.loading {
		return
	}
	d.loading = true
	a.job(func() func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: "Open With", ButtonLabel: "Open", DefaultPath: "/Applications", Filters: []mygo.FileFilter{{Name: "Applications", Extensions: []string{"app"}}}})
		return func() {
			if a.closed || a.modal != d {
				return
			}
			d.loading = false
			if err != nil {
				d.err = err.Error()
				return
			}
			if len(paths) > 0 {
				a.chooseApplication(d, paths[0])
			}
		}
	})
}

func (a *app) openWithView(c *ui.Context, d *dialog) {
	ui.Text(c, "Open With…").FontSize(16).Bold().TextAlign(ui.Center)
	if d.loading {
		ui.Text(c, "Loading applications…").TextColor(c.Theme().TextMuted)
	}
	_, height := c.Size()
	ui.Scroll(c).MaxHeight(max(float32(100), height-180)).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			for _, application := range d.applications {
				c.Key(application.Path + application.Name)
				ui.ButtonBase(c).Height(34).PaddingX(8).Radius(4).Justify(ui.Start).Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						if application.bitmap != nil {
							ui.Image(c, application.bitmap).Width(22).Height(22).Fit(ui.Contain)
						} else {
							ui.Text(c, "▣").Width(22).TextColor(c.Theme().TextMuted)
						}
						label := application.Name
						if application.Default {
							label += " (default)"
						}
						ui.Text(c, label).SingleLine().FontSize(14)
					})
				}).OnClick(func() { a.chooseApplication(d, application.Path) })
				if application.Default {
					ui.Box(c).Height(1).Background(c.Theme().Border).MarginY(5)
				}
			}
		})
	})
	if d.err != "" {
		ui.Text(c, d.err).TextColor(c.Theme().TextMuted)
	}
	ui.Box(c).Height(1).Background(c.Theme().Border)
	ui.Button(c, "All Applications…").Height(34).Justify(ui.Start).Disabled(d.loading).OnClick(func() { a.chooseOtherApplication(d) })
}
