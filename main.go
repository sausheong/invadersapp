package main

import (
	webview "github.com/webview/webview_go"
)

// windowScale scales the logical game frame (gameWidth x gameHeight, defined
// in gameplay code) up to a comfortable window size.
const windowScale = 2

// main wires up the webview window and its Go-side bindings, then starts the
// game. There is no web server: the page pulls rendered frames and pushes
// key events through webview_go's Bind, calling straight into Go.
func main() {
	loadSounds()

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("Space Invaders")
	w.SetSize(gameWidth*windowScale, gameHeight*windowScale, webview.HintFixed)

	// Bind must be called before SetHtml: the bound names need to exist as
	// soon as the page's own script starts running, and SetHtml loads (and
	// runs) that script immediately.
	w.Bind("keyDown", func(code int) { keyDown(code) })
	w.Bind("keyUp", func(code int) { keyUp(code) })
	w.Bind("frame", func(seq uint64) map[string]any {
		s, src := currentFrame()
		if s == seq {
			return map[string]any{"seq": s, "src": ""}
		}
		return map[string]any{"seq": s, "src": src}
	})

	// quitFunc is declared by the gameplay code; wire it up to a
	// thread-safe way of closing the window from any goroutine.
	quitFunc = func() { w.Dispatch(w.Terminate) }

	startGame()

	w.SetHtml(gameHTML())
	w.Run()
}
