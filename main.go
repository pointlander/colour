// Copyright 2026 The Colour Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"gitlab.com/gomidi/midi/writer"
)

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
