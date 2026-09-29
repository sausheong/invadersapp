package assets

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

// All three game sound assets (shoot, explosion, invaderkilled) are 11025 Hz
// mono 8-bit wav files, so the speaker is fixed at that rate. Any future
// asset with a different rate is resampled at load time in preloadSound.
const speakerSampleRate = beep.SampleRate(11025)

var (
	soundBuffers = map[string]*beep.Buffer{}
	speakerOnce  sync.Once
	speakerErr   error
)

// initSpeaker initializes the speaker exactly once for the whole process.
// Calling speaker.Init more than once just returns an error and does
// nothing, so it must only ever be called here.
func initSpeaker() error {
	speakerOnce.Do(func() {
		speakerErr = speaker.Init(speakerSampleRate, speakerSampleRate.N(time.Second/20))
	})
	return speakerErr
}

// preloadSound decodes the embedded "public/sounds/<name>.wav" asset once,
// resampling it to speakerSampleRate if needed, and stores the decoded audio
// in a beep.Buffer so every later play is just a cheap buffer read.
func preloadSound(name string) error {
	data, err := soundAsset(name)
	if err != nil {
		return fmt.Errorf("read embedded sound %q: %w", name, err)
	}

	streamer, format, err := wav.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode embedded sound %q: %w", name, err)
	}
	defer streamer.Close()

	var src beep.Streamer = streamer
	if format.SampleRate != speakerSampleRate {
		src = beep.Resample(4, format.SampleRate, speakerSampleRate, streamer)
	}

	buf := beep.NewBuffer(beep.Format{
		SampleRate:  speakerSampleRate,
		NumChannels: format.NumChannels,
		Precision:   format.Precision,
	})
	buf.Append(src)
	soundBuffers[name] = buf
	return nil
}

// LoadSounds initializes the speaker and preloads all game sounds. It's
// tolerant of failures (e.g. no audio device available) so the game can
// still be played, just silently, rather than crashing.
func LoadSounds() {
	if err := initSpeaker(); err != nil {
		fmt.Println("warning: could not init speaker, sound disabled:", err)
		return
	}
	for _, name := range []string{"shoot", "explosion", "invaderkilled"} {
		if err := preloadSound(name); err != nil {
			fmt.Println("warning: could not preload sound", name, ":", err)
		}
	}
}

// play a sound
func PlaySound(name string) {
	buf, ok := soundBuffers[name]
	if !ok {
		return
	}
	speaker.Play(buf.Streamer(0, buf.Len()))
}
