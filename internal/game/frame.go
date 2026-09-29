package game

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"sync/atomic"
)

var frameSeq atomic.Uint64
var frameData atomic.Value // string: "data:image/png;base64,..."

func init() {
	frameData.Store("")
}

// createFrame PNG-encodes img (BestSpeed, since this runs every tick at 50
// fps) and publishes it as the current frame, bumping the sequence number so
// CurrentFrame's caller can tell a new frame has arrived.
func createFrame(img image.Image) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return
	}
	frameData.Store("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()))
	frameSeq.Add(1)
}

// CurrentFrame returns the latest published frame and its sequence number.
// Thread-safe and cheap: no encoding happens here, just an atomic read.
func CurrentFrame() (seq uint64, src string) {
	return frameSeq.Load(), frameData.Load().(string)
}
