// Command invaders runs the Space Invaders game in a native webview window.
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	webview "github.com/webview/webview_go"

	"github.com/sausheong/invadersapp/internal/assets"
	"github.com/sausheong/invadersapp/internal/game"
)

// windowScale scales the logical game frame up to a comfortable window size.
const windowScale = 2

// Optional features compiled in with build tags (see jev.go) register
// functions to run once flags are parsed, and when the window closes.
var startHooks, stopHooks []func()

// main wires up the webview window and its Go-side bindings, then starts the
// game. There is no web server: the page pulls rendered frames and pushes
// key events through webview_go's Bind, calling straight into Go.
func main() {
	flag.Parse()
	for _, f := range startHooks {
		f()
	}

	assets.LoadSounds()

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("Space Invaders")
	w.SetSize(game.Width*windowScale, game.Height*windowScale, webview.HintFixed)

	// Bind must be called before SetHtml: the bound names need to exist as
	// soon as the page's own script starts running, and SetHtml loads (and
	// runs) that script immediately.
	w.Bind("keyDown", func(code int) { game.KeyDown(code) })
	w.Bind("keyUp", func(code int) { game.KeyUp(code) })
	w.Bind("frame", func(seq uint64) map[string]any {
		s, src := game.CurrentFrame()
		if s == seq {
			return map[string]any{"seq": s, "src": ""}
		}
		return map[string]any{"seq": s, "src": src}
	})

	// a thread-safe way of closing the window from any goroutine
	game.QuitFunc = func() { w.Dispatch(w.Terminate) }

	// Ctrl+C or kill closes the window normally, so the stop hooks still run.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		game.QuitFunc()
	}()

	game.Start()

	w.SetHtml(assets.GameHTML())
	w.Run()

	for _, f := range stopHooks {
		f()
	}
}
