# Space Invaders

A small cross-platform desktop Space Invaders game written in Go. The game renders every frame on the Go side and shows it in a native webview window, so there's no game engine or GUI toolkit involved — just images, a tiny web server and a browser view.

![Space Invaders](images/space-invaders.jpg)

The story behind the app, and a walkthrough of how it was built, is in the original article: [Create a simple cross-platform desktop game with Go](https://medium.com/sausheong/create-a-simple-cross-platform-desktop-game-with-go-8e5432128c9b).

![Space Invaders on a Mac](images/mac-invaders.gif)

## How to play

| Key | Action |
|---|---|
| `s` | Start the game (and play again after game over) |
| `←` / `→` | Move the laser cannon |
| `Space` | Fire |
| `q` | End the current game; on the game over screen, quit the app |

Aliens in the top row are worth 30 points, the middle row 20 and the bottom row 10. The game ends when an alien bomb hits your cannon or the aliens march down to your level.

## How it works

The app has three parts, all in one Go binary:

1. **A local web server** (`main.go`) listening on an ephemeral port on `127.0.0.1`. It serves the static pages in `public/` and three endpoints:
   * `/start` — starts the game loop and returns the game page
   * `/frame` — returns the latest frame as a base64 PNG data URI
   * `/key?event=<keyCode>` — passes a key press to the game loop
2. **The game loop** (`invaders.go`), running in its own goroutine. Each tick it moves the sprites, checks collisions, draws everything onto an image with [gift](https://github.com/disintegration/gift), and stores it as the current frame. The end screen text is drawn with `golang.org/x/image/font`.
3. **A native webview window** ([webview_go](https://github.com/webview/webview_go)) pointed at the local server. The page polls `/frame` and sends key presses back with plain JavaScript.

Sound effects are played with [beep](https://github.com/gopxl/beep) (`sound.go`). The sounds are loaded into memory once at startup.

```
.
├── main.go            web server, webview window, frame encoding
├── invaders.go        game loop, sprites, collisions
├── sound.go           sound loading and playback
├── invaders_test.go   unit tests
├── public/            HTML pages, sprite sheet, backgrounds, sounds
├── invaders.app/      macOS app bundle
└── build-macOS        builds the macOS app bundle
```

## Requirements

* Go 1.26 or later. The module pins `toolchain go1.26.8`, which Go downloads automatically if needed.
* cgo (`CGO_ENABLED=1`) and a C/C++ compiler, since webview wraps the operating system's own web view:
  * **macOS** — Xcode Command Line Tools (`xcode-select --install`).
  * **Linux** — GTK 3 and WebKitGTK development packages, e.g. `sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev` on Debian/Ubuntu.
  * **Windows** — the [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) runtime (preinstalled on Windows 10/11) and a compiler such as MinGW-w64.

## Build and run

The game loads its images and sounds from the `public/` directory next to the executable, so build a binary and run it from the project directory (`go run .` won't find the assets).

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

This builds the binary into `invaders.app/Contents/MacOS` and copies `public/` alongside it.

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
