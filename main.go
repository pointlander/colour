// Copyright 2026 The Colour Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math"

	"gitlab.com/gomidi/midi/writer"
)

const (
	N int = 8
)

var (
	c, cT [N][N]float64
)

func init() {
	for j := 0; j < N; j++ {
		c[0][j] = 1.0 / math.Sqrt(float64(N))
		cT[j][0] = c[0][j]
	}

	for i := 1; i < N; i++ {
		for j := 0; j < N; j++ {
			jj, ii := float64(j), float64(i)
			c[i][j] = math.Sqrt(2.0/8.0) * math.Cos(((2.0*jj+1.0)*ii*math.Pi)/(2.0*8.0))
			cT[j][i] = c[i][j]
		}
	}
}

func ForwardDCT(in *[N][N]uint8, out *[N][N]float64) {
	var x [N][N]float64
	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			for k := 0; k < N; k++ {
				x[i][j] += float64(int(in[i][k])-128) * cT[k][j]
			}
		}
	}

	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			y := 0.0
			for k := 0; k < N; k++ {
				y += c[i][k] * x[k][j]
			}
			out[i][j] = y
		}
	}
}

func main() {
	err := writer.WriteSMF("notes.mid", 1, func(wr *writer.SMF) error {
		for i := range 106 {
			note := uint8(i + 21)
			wr.SetChannel(0)
			writer.NoteOn(wr, note, 100)
			wr.SetDelta(120)
			writer.NoteOff(wr, note)
			wr.SetDelta(240)
		}

		writer.EndOfTrack(wr)

		return nil
	})
	if err != nil {
		panic(err)
	}
}
