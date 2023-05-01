package main

import (
	// "fmt"
	// "time"
	// "io"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto/v2"

	"github.com/unitoftime/flow/asset"
)

func LoadMp3(load *asset.Load, name string) *mp3.Decoder {
	// Open the file for reading. Do NOT close before you finish playing!
	file, err := load.Open(name)
	if err != nil {
		panic("opening failed: " + err.Error())
	}

	// Decode file. This process is done as the file plays so it won't
	// load the whole thing into memory.
	decodedMp3, err := mp3.NewDecoder(file)
	if err != nil {
		panic("mp3.NewDecoder failed: " + err.Error())
	}
	return decodedMp3
}

type AudioPlayer struct {
	ctx *oto.Context
	player oto.Player
}

func NewAudioPlayer() *AudioPlayer {
	// Usually 44100 or 48000. Other values might cause distortions in Oto
	samplingRate := 44100

	// Number of channels (aka locations) to play sounds from. Either 1 or 2.
	// 1 is mono sound, and 2 is stereo (most speakers are stereo).
	numOfChannels := 2

	// Bytes used by a channel to represent one sample. Either 1 or 2 (usually 2).
	audioBitDepth := 2

	// Remember that you should **not** create more than one context
	otoCtx, readyChan, err := oto.NewContext(samplingRate, numOfChannels, audioBitDepth)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	// It might take a bit for the hardware audio devices to be ready, so we wait on the channel.
	<-readyChan

	return &AudioPlayer{
		ctx: otoCtx,
	}
}

func (a *AudioPlayer) TogglePlayPause() {
	if a.player.IsPlaying() {
		a.player.Pause()
	} else {
		a.player.Play()
	}
}

func (a *AudioPlayer) Play(decoder *mp3.Decoder) {
	infLoop := NewInfiniteLoop(decoder, decoder.Length() - 10000) // TODO: I'm not sure why, but there seem to be some blank samples at the end or something, so I just trim those off (about 10k). Makes the loop better

	go func() {
		player := a.ctx.NewPlayer(infLoop)
		a.player = player

		// Play starts playing the sound and returns without waiting for it (Play() is async).
		player.Play()

		// for player.IsPlaying() {
		// 	time.Sleep(1 * time.Millisecond)
		// }
	}()
}
