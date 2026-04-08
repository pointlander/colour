// Copyright 2026 The Colour Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io"
	"math"
	"time"

	"github.com/ebitengine/oto/v3"
)

// Key is a piano key
type Key struct {
	Key      float64
	Buffer   bytes.Buffer
	Length   int64
	Duration time.Duration
}

func NewKey(key float64, duration time.Duration) *Key {
	length := 2 * int64(48000) * int64(duration) / int64(time.Second)
	length = length / 4 * 4
	return &Key{
		Key:      key,
		Length:   length,
		Duration: duration,
	}
}

func (s *Key) Read(buf []byte) (int, error) {
	if s.Buffer.Cap() == 0 {
		length := float64(48000) / float64(s.Key)
		length1 := float64(48000) / float64(2*s.Key)
		length2 := float64(48000) / float64(3*s.Key)
		k := float64(s.Length) / 8
		for i := 0; i < int(s.Length)/4; i++ {
			const max = 32767
			envelope := math.Exp(-float64(i) / k)
			b := int16(math.Sin(2*math.Pi*float64(i)/length) * 0.1 * max * envelope)
			b += int16(math.Sin(2*math.Pi*float64(i)/length1) * 0.1 * max * envelope)
			b += int16(math.Sin(2*math.Pi*float64(i)/length2) * 0.1 * max * envelope)
			for ch := 0; ch < 2; ch++ {
				s.Buffer.Write([]byte{byte(b)})
				s.Buffer.Write([]byte{byte(b >> 8)})
			}
		}
	}

	if s.Buffer.Len() > 0 {
		n, err := s.Buffer.Read(buf)
		if err != nil {
			panic(err)
		}
		return n, nil
	}

	return 0, io.EOF
}

// Piano is a piano
type Piano struct {
	Context *oto.Context
}

// NewPiano crates a new piano
func NewPiano() Piano {
	op := &oto.NewContextOptions{}
	op.SampleRate = 48000
	op.ChannelCount = 2
	op.Format = oto.FormatSignedInt16LE
	context, ready, err := oto.NewContext(op)
	if err != nil {
		panic(err)
	}
	<-ready
	return Piano{
		Context: context,
	}
}

// Play plays a key
func (p *Piano) Play(key *Key) {
	player := p.Context.NewPlayer(key)
	player.Play()
	time.Sleep(key.Duration)
}
