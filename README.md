# Space Invaders

A small cross-platform desktop Space Invaders game written in Go. The game renders every frame on the Go side and shows it in a native webview window — no game engine, no GUI toolkit, and (as of v0.3.0) no web server either: the window talks straight to Go through [webview_go](https://github.com/webview/webview_go)'s `Bind`.

![Space Invaders](images/space-invaders.jpg)

The story behind the app, and a walkthrough of how it was built, is in the original article: [Create a simple cross-platform desktop game with Go](https://medium.com/sausheong/create-a-simple-cross-platform-desktop-game-with-go-8e5432128c9b).

![Space Invaders on a Mac](images/mac-invaders.gif)

## How to play

| Key | Action |
|---|---|
| `s` | Start the game, and restart after game over |
| `←` / `→` | Move the laser cannon (hold to keep moving) |
| `Space` | Fire |
| `p` | Pause / resume |
| `q` | Quit to the title screen while playing; quit the app from the title screen |

Aliens in the top row are worth 30 points, the middle row 20 and the bottom row 10. You have multiple lives per game, and the aliens come in waves — clear one and a faster, tougher wave marches in behind it. Destructible shields give you cover from alien bombs, and a bonus UFO occasionally crosses the top of the screen for extra points. The game keeps a running high score across plays. It's over for good when you lose your last life.

## How it works

The whole game is one Go binary with **no web server and no browser involved** — just a native OS webview window bound directly to Go functions:

1. **The game loop** (`invaders.go` and friends), running in its own goroutine, owns all game state. Each tick it moves sprites, checks collisions, draws everything onto an image with [gift](https://github.com/disintegration/gift), and encodes it as a PNG data URI, tagged with an incrementing sequence number.
2. **A native webview window** (`main.go`, via [webview_go](https://github.com/webview/webview_go)) hosts a tiny embedded HTML page (`public/html/game.html`) and binds three Go functions straight into its JavaScript global scope with `w.Bind`:
   * `frame(lastSeq)` — the page calls this in a `requestAnimationFrame` loop; Go returns the current sequence number and, only when it has changed, the new frame as a data URI
   * `keyDown(code)` / `keyUp(code)` — the page's `keydown`/`keyup` listeners call these directly with the JS key code
   Each bound call crosses into Go and back as a JSON-marshalled Promise; there's no HTTP, no polling endpoint, and no port to manage.
3. **Assets are embedded** (`assets.go`, via `//go:embed public`) — the sprite sheet, backgrounds, sounds and the HTML page all live inside the compiled binary, decoded on demand. That means `go run .` works from a source checkout, and a distributed binary or app bundle needs no `public/` folder alongside it.

Sound effects are played with [beep](https://github.com/gopxl/beep) (`sound.go`), reading their `.wav` data out of the same embedded filesystem. The sounds are decoded into memory once at startup.

```
.
├── main.go              webview window, Bind wiring, quit handling
├── assets.go            //go:embed public + asset accessors (images, HTML, sounds)
├── sound.go             sound loading and playback
├── invaders.go          Sprite type, collide, createAlien
├── game.go              Game state, startNewGame/startNextWave, step() and the game loop
├── frame.go             frame encoding (createFrame) and publishing (currentFrame)
├── input.go             keyDown/keyUp and the thread-safe input snapshot
├── render.go            render(): draws the current Game state to an image
├── hud.go               printLine (bitmap text) and drawHUD (score/lives)
├── shield.go            destructible bunker bitmap logic
├── highscore.go         high score load/save (JSON in the user config dir)
├── invaders_test.go     unit tests for Sprite/collide
├── game_test.go         unit tests for game state and step()
├── render_test.go       snapshot rendering tests (INVADERS_SNAPSHOT_DIR)
├── shield_test.go       unit tests for shields
├── highscore_test.go    unit tests for high score persistence
├── public/              game HTML page, sprite sheet, backgrounds, sounds
├── invaders.app/        macOS app bundle
└── build-macOS          builds the macOS app bundle
```

## Requirements

* Go 1.26 or later. The module pins `toolchain go1.26.8`, which Go downloads automatically if needed.
* cgo (`CGO_ENABLED=1`) and a C/C++ compiler, since webview wraps the operating system's own web view:
  * **macOS** — Xcode Command Line Tools (`xcode-select --install`).
  * **Linux** — GTK 3 and WebKitGTK development packages, e.g. `sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev` on Debian/Ubuntu.
  * **Windows** — the [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) runtime (preinstalled on Windows 10/11) and a compiler such as MinGW-w64.

## Build and run

All assets are embedded in the binary, so there's nothing to copy alongside it — `go run .` works straight from a checkout:

```sh
go run .
```

To build a standalone binary:

```sh
go build -o invaders
./invaders
```

On Windows, hide the console window with:

```sh
go build -ldflags="-H windowsgui" -o invaders.exe
```

### macOS app bundle

```sh
./build-macOS
open invaders.app
```

This just builds the binary into `invaders.app/Contents/MacOS` — since assets are embedded, the bundle needs nothing else alongside it.

A prebuilt Apple Silicon bundle is attached to each [release](https://github.com/sausheong/invadersapp/releases). It is unsigned, so right-click it and choose **Open** the first time.

## Tests

```sh
go test ./...
```

## Screenshots

The start screen on Windows:

![Space Invaders on Windows](images/win-invaders.png)

Playing on Windows:

![Space Invaders game on Windows](images/win-invaders.gif)

## Credits

* Sound effects from [Classics United](http://www.classicgaming.cc/classics/space-invaders/sounds).
* Thanks to Ibrahim Wu, who helped debug the app on Windows and found the MSHTML frame caching problem.
* Thanks to Serge Zaitsev for the original [webview](https://github.com/zserge/webview) package, and to the maintainers of [webview_go](https://github.com/webview/webview_go) and [gopxl/beep](https://github.com/gopxl/beep).
