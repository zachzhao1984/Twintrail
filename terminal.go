package main

import (
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

type terminalSession interface {
	View(*ui.Context) ui.Element
	SetDir(string, uint64) error
	SetFont(fontSetting)
	Close() error
}
type tabTerminal struct {
	visible, starting bool
	generation        int
	revision          uint64
	session           terminalSession
	err               string
	handle            ui.Handle
}
type nativeTerminal struct {
	term   *terminal.Terminal
	bridge *cwdBridge
	font   fontSetting
}

func (n *nativeTerminal) View(c *ui.Context) ui.Element        { return terminal.View(c, n.term) }
func (n *nativeTerminal) SetDir(path string, rev uint64) error { return n.bridge.setDir(path, rev) }
func (n *nativeTerminal) Close() error                         { err := n.term.Close(); n.bridge.close(); return err }
func newTerminal(path string) (terminalSession, error) {
	bridge, err := newCWDBridge(path)
	if err != nil {
		return nil, err
	}
	theme := terminal.DarkTheme()
	theme.Background = ui.Hex("#1c232b")
	theme.Foreground = ui.Hex("#d6dae0")
	term, err := terminal.New(terminal.Options{Command: []string{"/bin/zsh", "-l"}, Dir: path, Env: bridge.env(), Theme: theme, Font: terminal.Font{Size: 12}, Cursor: terminal.CursorBar})
	if err != nil {
		bridge.close()
		return nil, err
	}
	return &nativeTerminal{term: term, bridge: bridge, font: defaultFonts().Terminal}, nil
}
func (a *app) toggleTerminal() {
	if a.modal != nil {
		return
	}
	t := a.current()
	t.terminal.visible = !t.terminal.visible
	if t.terminal.visible {
		a.focus = a.active + 3
		a.ensureTerminal(t)
	} else {
		a.focus = a.active + 1
	}
	a.refreshTerminalMenu()
}
func (a *app) ensureTerminal(t *tab) {
	s := &t.terminal
	if s.session != nil || s.starting {
		return
	}
	s.generation++
	generation := s.generation
	s.starting = true
	s.err = ""
	path := t.path
	factory := a.newTerminal
	if factory == nil {
		factory = newTerminal
	}
	a.terminalJobs.Add(1)
	a.job(func() func() {
		defer a.terminalJobs.Done()
		session, err := factory(path)
		if session != nil {
			a.terminalSessions.Store(session, struct{}{})
		}
		return func() {
			if a.closed || generation != s.generation {
				if session != nil {
					a.releaseTerminal(session)
				}
				return
			}
			s.starting = false
			if err != nil {
				s.err = err.Error()
				return
			}
			s.session = session
			session.SetFont(a.fonts.Terminal)
			a.syncTerminal(t)
		}
	})
}
func (a *app) syncTerminal(t *tab) {
	s := &t.terminal
	if s.session == nil {
		return
	}
	s.revision++
	revision := s.revision
	session := s.session
	path := t.path
	generation := s.generation
	a.job(func() func() {
		err := session.SetDir(path, revision)
		return func() {
			if !a.closed && generation == s.generation && revision == s.revision && err != nil {
				s.err = err.Error()
			}
		}
	})
}
func (a *app) closeTerminal(t *tab) {
	s := &t.terminal
	s.generation++
	session := s.session
	s.session = nil
	s.starting = false
	s.visible = false
	if session != nil {
		a.releaseTerminal(session)
	}
}
func (a *app) releaseTerminal(session terminalSession) {
	a.terminalJobs.Add(1)
	a.job(func() func() {
		defer a.terminalJobs.Done()
		session.Close()
		a.terminalSessions.Delete(session)
		return func() {}
	})
}

// Called after the event loop ends, including sessions whose creation
// finished after the window closed and could not publish a UI callback.
func (a *app) finishTerminals() {
	a.terminalJobs.Wait()
	a.terminalSessions.Range(func(key, value any) bool { key.(terminalSession).Close(); a.terminalSessions.Delete(key); return true })
}
func (a *app) closeTerminals() {
	for i := range a.panes {
		for _, t := range a.panes[i].tabs {
			a.closeTerminal(t)
		}
	}
}
func (a *app) terminalView(c *ui.Context) {
	t := a.current()
	s := &t.terminal
	_, height := c.Size()
	c.Key(fmt.Sprintf("terminal-%p", t))
	ui.Column(c).Height(min(240, max(120, height*0.4))).Shrink(0).Background(ui.Hex("#1c232b")).Border(1, c.Theme().Border).BorderWidth(1, 0, 0, 0).Children(func() {
		ui.Row(c).Height(24).PaddingX(6).Gap(8).AlignItems(ui.Center).Background(c.Theme().Background).Children(func() {
			ui.Text(c, "Terminal").Bold()
			ui.Text(c, t.path).Grow(1).MinWidth(0).SingleLine().TextColor(c.Theme().TextMuted)
			ui.ButtonBase(c).Label("Close Terminal").Children(func() { ui.Text(c, "×").FontSize(16) }).OnClick(a.toggleTerminal)
		})
		if s.starting {
			ui.Text(c, "Starting terminal…").Padding(8)
		} else if s.err != "" {
			ui.Text(c, s.err).Padding(8).TextColor(c.Theme().Danger)
			ui.Button(c, "Retry Terminal").OnClick(func() { a.closeTerminal(t); t.terminal.visible = true; a.ensureTerminal(t) })
		} else if s.session != nil {
			c.Key("shell")
			s.session.View(c).Grow(1).FillWidth().Bind(&s.handle).FocusBind(&a.focus, a.active+3).OnShortcut(ui.Cmd, ui.KeyBackquote, a.toggleTerminal)
		}
	})
}

// updateMenuItem builds the updater's "Check for Updates…" item. Building it
// reads the user's language, which asks the app's event loop, so it is only
// safe once the loop runs: the app builds its menu from WhenReady. A test
// has no loop, and overrides this with an item that does not read one.
var updateMenuItem = updater.MenuItem

func (a *app) applicationMenu() *mygo.Menu {
	a.terminalMenu = &mygo.MenuItem{ID: "toggle-terminal", Label: "Toggle Terminal", Type: mygo.MenuItemCheckbox, Accelerator: "Cmd+`", Click: func(item *mygo.MenuItem, w *mygo.Window) {
		a.toggleTerminal()
		if w != nil {
			w.Invalidate()
		}
	}}
	return mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{{Role: mygo.RoleAbout}, updateMenuItem(), mygo.Separator(), a.preferenceMenu(), mygo.Separator(), {Role: mygo.RoleServices}, mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(), {Role: mygo.RoleQuit}}}, {Role: mygo.RoleFileMenu}, {Role: mygo.RoleEditMenu}, {Label: "View", Submenu: []*mygo.MenuItem{a.terminalMenu, mygo.Separator(), {Role: mygo.RoleToggleFullScreen}}}, {Role: mygo.RoleWindowMenu}})
}
func (a *app) refreshTerminalMenu() {
	if a.terminalMenu != nil && a.terminalMenu.IsChecked() != a.current().terminal.visible {
		a.terminalMenu.SetChecked(a.current().terminal.visible)
	}
}

func (n *nativeTerminal) SetFont(f fontSetting) {
	if n.font == f {
		return
	}
	family, weight := f.familyWeight()
	n.term.SetFont(terminal.Font{Family: family, Weight: weight, Size: float32(f.Size)})
	n.font = f
}
