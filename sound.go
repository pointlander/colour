// Copyright 2026 The Colour Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/ebitengine/oto/v3"
)

type Key struct {
	key          float64
	channelCount int
	length       int64
	format       oto.Format
	buffer       bytes.Buffer
}

func NewKey(key float64, duration time.Duration) *Key {
	length := 2 * int64(48000) * int64(duration) / int64(time.Second)
	length = length / 4 * 4
	return &Key{
		key:          key,
		length:       length,
		channelCount: 2,
		format:       oto.FormatSignedInt16LE,
	}
}

func (s *Key) Read(buf []byte) (int, error) {
	if s.buffer.Cap() == 0 {
		length := float64(48000) / float64(s.key)
		length1 := float64(48000) / float64(2*s.key)
		length2 := float64(48000) / float64(3*s.key)
		k := float64(s.length) / 8
		fmt.Println(1 / k)
		for i := 0; i < int(s.length)/4; i++ {
			const max = 32767
			envelope := math.Exp(-float64(i) / k)
			b := int16(math.Sin(2*math.Pi*float64(i)/length) * 0.1 * max * envelope)
			b += int16(math.Sin(2*math.Pi*float64(i)/length1) * 0.1 * max * envelope)
			b += int16(math.Sin(2*math.Pi*float64(i)/length2) * 0.1 * max * envelope)
			for ch := 0; ch < 2; ch++ {
				s.buffer.Write([]byte{byte(b)})
				s.buffer.Write([]byte{byte(b >> 8)})
			}
		}
	}

	if s.buffer.Len() > 0 {
		n, err := s.buffer.Read(buf)
		if err != nil {
			panic(err)
		}
		return n, nil
	}

	return 0, io.EOF
}
