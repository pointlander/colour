// Copyright 2026 The Colour Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"math/rand"
	"os"

	"github.com/pointlander/colour/pagerank"

	"github.com/nfnt/resize"
	"gitlab.com/gomidi/midi/writer"
)

const (
	// N is the block size
	N int = 8
	// Scale is the scale size
	Scale int = 4
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

// ForwardDCT computes the forward dct
func ForwardDCT(in *[N][N]uint8, out *[N * N]float64) {
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
			out[i*N+j] = y
		}
	}
}

// Dot is the dot product
func Dot(a, b *[N * N]float64) float64 {
	sum := 0.0
	for i, value := range a {
		sum += value * b[i]
	}
	return sum
}

// CS implements cosine similarity
func CS(a, b *[N * N]float64) float64 {
	ab := Dot(a, b)
	aa := Dot(a, a)
	bb := Dot(b, b)
	if aa == 0 || bb == 0 {
		return 0
	}
	return ab / (math.Sqrt(aa) * math.Sqrt(bb))
}

// Colour
type Colour struct {
	R, G, B uint32
	Note    uint8
}

var Notes = []Colour{
	{0xffff, 0, 0, 62},
	{0xffff, 0xa5a5, 0, 64},
	{0xffff, 0xffff, 0, 65},
	{0, 0xffff, 0, 67},
	{0, 0, 0xffff, 69},
	{0x4b4b, 0, 0x8282, 71},
	{0x7f00, 0, 0xffff, 60},
}

func main() {
	rng := rand.New(rand.NewSource(1))
	input, err := os.Open("images/image01.png")
	if err != nil {
		panic(err)
	}
	defer input.Close()
	img, _, err := image.Decode(input)
	if err != nil {
		panic(err)
	}
	img = resize.Resize(uint(img.Bounds().Max.X/Scale), uint(img.Bounds().Max.Y/Scale), img, resize.NearestNeighbor)
	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y
	type Entry struct {
		DCT  [N * N]float64
		Rank float64
		Note uint8
	}
	entries := make([]Entry, (width/N)*(height/N))
	fmt.Println(width/N, height/N)
	var g [8][8]uint8
	index := 0
	for r := 0; r < height/N; r++ {
		for c := 0; c < width/N; c++ {
			colors := make([]float64, len(Notes))
			for y := 0; y < N; y++ {
				for x := 0; x < N; x++ {
					clr := img.At(c*N+x, r*N+y)
					for z := range Notes {
						r, g, b, _ := clr.RGBA()
						red := float64(Notes[z].R) - float64(r)
						green := float64(Notes[z].G) - float64(g)
						blue := float64(Notes[z].B) - float64(b)
						colors[z] += math.Sqrt(red*red + green*green + blue*blue)
					}
					gray := color.GrayModel.Convert(clr).(color.Gray)
					g[y][x] = gray.Y
				}
			}
			max, idx := 0.0, 0
			for z := range colors {
				if colors[z] > max {
					max, idx = colors[z], z
				}
			}
			entries[index].Note = Notes[idx].Note
			ForwardDCT(&g, &entries[index].DCT)
			index++
		}
	}
	graph := pagerank.NewGraph(len(entries), rng)
	for i := range entries {
		for j := range entries {
			graph.Link(uint32(i), uint32(j), math.Abs(CS(&entries[i].DCT, &entries[j].DCT)))
		}
	}
	graph.Rank(1, .000001, func(id int, rank float64) {
		entries[id].Rank = rank
	})
	for i := range entries {
		fmt.Println(i, entries[i].Rank, entries[i].Note)
	}

	err = writer.WriteSMF("notes.mid", 1, func(wr *writer.SMF) error {
		for range 33 {
			total, selected, index := 0.0, rng.Float64(), 0
			for j := range entries {
				total += entries[j].Rank
				if selected < total {
					index = j
					break
				}
			}
			wr.SetChannel(0)
			writer.NoteOn(wr, entries[index].Note, 100)
			wr.SetDelta(120)
			writer.NoteOff(wr, entries[index].Note)
			wr.SetDelta(240)
		}

		writer.EndOfTrack(wr)

		return nil
	})
	if err != nil {
		panic(err)
	}
}
