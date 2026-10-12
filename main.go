package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	"log"
	"os"
)

func main() {
	// native draws the update window in the same native UI as the rest of
	// the app, so the app needs no webview. Use one of the two updater
	// plugins only.
	mygo.Use(native.Plugin)
	home, _ := os.UserHomeDir()
	a := newApp(home, home)
	a.restore()
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Twintrail", Width: 1240, Height: 760, MinWidth: 960, MinHeight: 420, StateKey: "main", TitleBarStyle: mygo.TitleBarHidden, TrafficLightPosition: &mygo.Point{X: 10, Y: 8}, BackgroundColor: "#222a32", Content: ui.View(a.view)})
		mygo.App.SetMenu(a.applicationMenu())
		a.update = func(fn func()) {
			w.Update(func() {
				fn()
			})
		}
		w.OnClose(func(e *mygo.CloseEvent) { a.persistSettings(false) })
		w.OnClosed(func() { a.closed = true; a.closeTerminals() })
		for i := range a.panes {
			for _, t := range a.panes[i].tabs {
				a.load(t, t.path)
			}
		}
	})
	err := mygo.App.Run()
	a.finishTerminals()
	if err != nil {
		log.Fatal(err)
	}
}
