package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/draw"
	_ "image/png" // register the PNG decoder used by image.Decode below
	"log"
)

// publicFS embeds every asset the game needs (images, sounds, and the game
// HTML page) so the binary is self-contained: no files need to sit next to
// the executable, "go run ." works, and app bundles don't need a copied
// public/ directory.
//
//go:embed public
var publicFS embed.FS

// assetImage decodes the PNG at path (relative to public/, e.g.
// "images/sprites.png") from the embedded filesystem and returns a fresh
// *image.RGBA copy. A new copy is returned on every call so callers can draw
// on it freely without racing or corrupting anyone else's copy; it's fatal
// (log.Fatalf) if the asset is missing or not a valid image, since a missing
// built-in asset means the binary itself is broken.
func assetImage(path string) image.Image {
	full := "public/" + path
	data, err := publicFS.ReadFile(full)
	if err != nil {
		log.Fatalf("assetImage: cannot read embedded asset %s: %v", full, err)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Fatalf("assetImage: cannot decode embedded asset %s: %v", full, err)
	}

	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	return dst
}

// gameHTML returns the single embedded HTML page that the webview window
// loads via SetHtml.
func gameHTML() string {
	data, err := publicFS.ReadFile("public/html/game.html")
	if err != nil {
		log.Fatalf("gameHTML: cannot read embedded public/html/game.html: %v", err)
	}
	return string(data)
}

// soundAsset returns the raw bytes of the embedded
// "public/sounds/<name>.wav" file.
func soundAsset(name string) ([]byte, error) {
	return publicFS.ReadFile(fmt.Sprintf("public/sounds/%s.wav", name))
}
