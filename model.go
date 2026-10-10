package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type entry struct {
	name     string
	size     int64
	dir      bool
	modified time.Time
}
type tab struct {
	terminal   tabTerminal
	path       string
	entries    []entry
	cursor     int
	marked     map[string]bool
	hidden     bool
	filter     string
	list       ui.ListState
	generation int
	loading    bool
	err        string
	disk       string
	searchOpen bool
	descending bool
}
type pane struct {
	tabs    []*tab
	current int
}
type dialog struct {
	kind, value  string
	open         bool
	sources      []string
	target       string
	input        ui.Handle
	applications []openWithApplication
	loading      bool
	err          string
}
type app struct {
	discoverApplications func([]string) ([]openWithApplication, error)
	loadApplicationIcon  func(string) (*ui.Bitmap, error)
	applicationLists     map[string]cachedApplications
	applicationJobs      map[string]bool
	applicationIcons     map[string]*ui.Bitmap
	applicationIconJobs  map[string]bool
	openApplication      func(string, []string)
	fonts                fontSettings
	settingsWriter       sessionWriter
	panes                [2]pane
	active               int
	focus                int
	split                float32
	lastWidth            float32
	modal                *dialog
	status               string
	busy                 bool
	light                bool
	update               func(func())
	closed               bool
	newTerminal          func(string) (terminalSession, error)
	terminalMenu         *mygo.MenuItem
	terminalJobs         sync.WaitGroup
	terminalSessions     sync.Map
}

func newApp(left, right string) *app {
	a := &app{fonts: defaultFonts(), split: 550, lastWidth: 1100, focus: 1, status: "Ready"}
	for i, p := range []string{left, right} {
		t := &tab{path: p, marked: map[string]bool{}}
		a.panes[i] = pane{tabs: []*tab{t}}
	}
	return a
}
func (a *app) current() *tab { p := &a.panes[a.active]; return p.tabs[p.current] }
func readDirectory(path string, hidden bool) ([]entry, error) {
	ds, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := []entry{}
	if filepath.Dir(path) != path {
		out = append(out, entry{name: "..", dir: true})
	}
	for _, d := range ds {
		if !hidden && strings.HasPrefix(d.Name(), ".") {
			continue
		}
		info, e := d.Info()
		if e != nil {
			continue
		}
		out = append(out, entry{d.Name(), info.Size(), d.IsDir(), info.ModTime()})
	}
	sortEntries(out, false)
	return out, nil
}
func sortEntries(es []entry, desc bool) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i], es[j]
		if a.name == ".." {
			return true
		}
		if b.name == ".." {
			return false
		}
		if a.dir != b.dir {
			return a.dir
		}
		if desc {
			return strings.ToLower(a.name) > strings.ToLower(b.name)
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
}
func (a *app) job(work func() func()) {
	if a.update == nil {
		work()()
		return
	}
	publish := a.update
	go func() { done := work(); publish(done) }()
}
func (a *app) load(t *tab, path string) {
	path = filepath.Clean(path)
	t.generation++
	generation := t.generation
	hidden := t.hidden
	t.loading = true
	a.job(func() func() {
		es, err := readDirectory(path, hidden)
		disk := diskSpace(path)
		return func() {
			if a.closed || generation != t.generation {
				return
			}
			t.loading = false
			if err != nil {
				t.err = err.Error()
				a.status = t.err
				return
			}
			t.path = path
			t.disk = disk
			t.entries = es
			sortEntries(t.entries, t.descending)
			t.cursor = 0
			t.marked = map[string]bool{}
			t.filter = ""
			t.err = ""
			t.list.ScrollTo(0, ui.Start)
			a.syncTerminal(t)
		}
	})
}
func (t *tab) visible() []entry {
	if t.filter == "" {
		return t.entries
	}
	out := []entry{}
	for _, e := range t.entries {
		if strings.Contains(strings.ToLower(e.name), strings.ToLower(t.filter)) {
			out = append(out, e)
		}
	}
	return out
}
func (a *app) open() {
	t := a.current()
	es := t.visible()
	if t.cursor < 0 || t.cursor >= len(es) {
		return
	}
	e := es[t.cursor]
	path := filepath.Join(t.path, e.name)
	if e.dir {
		a.load(t, path)
	} else {
		a.launch("open", path)
	}
}
func (a *app) launch(command string, args ...string) {
	a.job(func() func() {
		err := exec.Command(command, args...).Run()
		return func() {
			if err != nil {
				a.status = err.Error()
			}
		}
	})
}
func (a *app) targets() []string {
	t := a.current()
	out := []string{}
	for _, e := range t.entries {
		if e.name != ".." && t.marked[e.name] {
			out = append(out, filepath.Join(t.path, e.name))
		}
	}
	if len(out) == 0 {
		es := t.visible()
		if t.cursor >= 0 && t.cursor < len(es) && es[t.cursor].name != ".." {
			out = append(out, filepath.Join(t.path, es[t.cursor].name))
		}
	}
	return out
}
func (a *app) ask(kind string) {
	t := a.current()
	d := &dialog{open: true, kind: kind, sources: a.targets(), target: t.path}
	if (kind == "Copy" || kind == "Move" || kind == "Rename" || kind == "Move to Trash" || kind == "Delete") && len(d.sources) == 0 {
		a.status = "Select a file or folder first"
		return
	}
	switch kind {
	case "Copy", "Move":
		p := &a.panes[1-a.active]
		d.value = p.tabs[p.current].path
	case "Rename":
		if len(d.sources) != 1 {
			return
		}
		d.value = filepath.Base(d.sources[0])
	case "Go to path":
		d.value = t.path
	case "New folder":
		d.value = "New folder"
	}
	a.modal = d
}
func (a *app) confirm() {
	d := a.modal
	if d == nil {
		return
	}
	if d.kind == "Go to path" {
		path := d.value
		if strings.HasPrefix(path, "~/") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[2:])
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(d.target, path)
		}
		a.load(a.current(), path)
		a.modal = nil
		return
	}
	if a.busy {
		return
	}
	a.busy = true
	a.modal = nil
	a.status = d.kind + "…"
	a.job(func() func() {
		err := perform(d)
		return func() {
			a.busy = false
			if err != nil {
				a.status = err.Error()
			} else {
				a.status = d.kind + " completed"
			}
			for i := range a.panes {
				for _, t := range a.panes[i].tabs {
					a.load(t, t.path)
				}
			}
		}
	})
}
func perform(d *dialog) error {
	switch d.kind {
	case "Delete":
		for _, src := range d.sources {
			if err := os.RemoveAll(src); err != nil {
				return err
			}
		}
		return nil
	case "New folder":
		if d.value == "" || filepath.IsAbs(d.value) || strings.HasPrefix(filepath.Clean(d.value), "..") {
			return fmt.Errorf("Enter a relative folder name")
		}
		return os.MkdirAll(filepath.Join(d.target, d.value), 0755)
	case "Rename":
		if d.value == "" || filepath.Base(d.value) != d.value || d.value == "." || d.value == ".." {
			return fmt.Errorf("Invalid name")
		}
		target := filepath.Join(d.target, d.value)
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("Destination already exists")
		}
		return os.Rename(d.sources[0], target)
	case "Copy", "Move", "Move to Trash":
		target := d.value
		if d.kind == "Move to Trash" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			target = filepath.Join(home, ".Trash")
		}
		info, err := os.Stat(target)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("Destination must be a folder")
		}
		for _, src := range d.sources {
			dst := filepath.Join(target, filepath.Base(src))
			if d.kind == "Move to Trash" {
				dst = filepath.Join(target, fmt.Sprintf("%s-%d", filepath.Base(src), time.Now().UnixNano()))
			}
			if err := validateDestination(src, dst); err != nil {
				return err
			}
			if d.kind == "Copy" {
				err = copyEntry(src, dst)
			} else {
				err = os.Rename(src, dst)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("Unknown operation")
}
func validateDestination(src, dst string) error {
	srcAbs, _ := filepath.Abs(src)
	dstAbs, _ := filepath.Abs(dst)
	parent, err := filepath.EvalSymlinks(filepath.Dir(dstAbs))
	if err != nil {
		return err
	}
	dstAbs = filepath.Join(parent, filepath.Base(dstAbs))
	if resolved, e := filepath.EvalSymlinks(srcAbs); e == nil {
		srcAbs = resolved
	}
	rel, err := filepath.Rel(srcAbs, dstAbs)
	if err == nil && (rel == "." || !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..") {
		return fmt.Errorf("Cannot transfer a folder into itself")
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		return fmt.Errorf("Destination exists: %s", dst)
	}
	return nil
}
func copyEntry(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, e := os.Readlink(src)
		if e != nil {
			return e
		}
		return os.Symlink(link, dst)
	}
	if info.IsDir() {
		if err = os.Mkdir(dst, info.Mode().Perm()); err != nil {
			return err
		}
		ds, e := os.ReadDir(src)
		if e != nil {
			return e
		}
		for _, d := range ds {
			if e = copyEntry(filepath.Join(src, d.Name()), filepath.Join(dst, d.Name())); e != nil {
				return e
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Unsupported file: %s", src)
	}
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	closeErr := out.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func sizeText(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	for _, u := range []struct {
		size  int64
		label string
	}{{1 << 30, "GB"}, {1 << 20, "MB"}, {1 << 10, "KB"}} {
		if n >= u.size {
			return fmt.Sprintf("%.1f %s", float64(n)/float64(u.size), u.label)
		}
	}
	return ""
}
