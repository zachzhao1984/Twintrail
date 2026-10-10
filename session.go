package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type savedTab struct {
	Path   string
	Hidden bool
}
type savedPane struct {
	Tabs    []savedTab
	Current int
}
type session struct {
	Panes  [2]savedPane
	Active int
	Split  float32
	Width  float32
	Light  bool
	Fonts  fontSettings `json:"fonts"`
}

func sessionPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Twintrail", "session.json")
}

// The old directory is read only when the new session does not exist.
// Saving always uses Twintrail, so existing preferences migrate on exit.
const legacyApplicationName = "mygo-commander"

func readSessionData(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if !errors.Is(err, os.ErrNotExist) || path == "" {
		return data, err
	}
	legacy := filepath.Join(filepath.Dir(filepath.Dir(path)), legacyApplicationName, "session.json")
	return os.ReadFile(legacy)
}

func (a *app) restore() {
	data, err := readSessionData(sessionPath())
	if err != nil {
		return
	}
	var s session
	if json.Unmarshal(data, &s) != nil {
		return
	}
	for i, p := range s.Panes {
		tabs := []*tab{}
		for _, t := range p.Tabs {
			if !filepath.IsAbs(t.Path) {
				continue
			}
			tabs = append(tabs, &tab{path: t.Path, hidden: t.Hidden, marked: map[string]bool{}})
		}
		if len(tabs) > 0 {
			a.panes[i] = pane{tabs: tabs}
			if p.Current >= 0 && p.Current < len(tabs) {
				a.panes[i].current = p.Current
			}
		}
	}
	if s.Active >= 0 && s.Active < 2 {
		a.active = s.Active
		a.focus = s.Active + 1
	}
	if s.Split >= 180 {
		a.split = s.Split
	}
	if s.Width > 0 {
		a.lastWidth = s.Width
	}
	a.light = s.Light
	a.fonts = s.Fonts.normalized()
}
func (a *app) snapshotSession() session {
	s := session{Active: a.active, Split: a.split, Width: a.lastWidth, Light: a.light, Fonts: a.fonts}
	for i, p := range a.panes {
		s.Panes[i].Current = p.current
		for _, t := range p.tabs {
			s.Panes[i].Tabs = append(s.Panes[i].Tabs, savedTab{t.path, t.hidden})
		}
	}
	return s
}
func saveSession(s session) {
	path := sessionPath()
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "session-*")
	if err != nil {
		return
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return
	}
	if tmp.Close() != nil {
		return
	}
	os.Rename(name, path)
}
