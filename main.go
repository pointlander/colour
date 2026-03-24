// Copyright 2026 The Colour Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"math/rand"
	"os"

	"github.com/pointlander/colour/kmeans"
	"github.com/pointlander/colour/pagerank"

	"github.com/nfnt/resize"
	"gitlab.com/gomidi/midi/writer"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
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

const (
	// Size is the size of the universe
	Size = 128
	// Order is the order of the markov model
	Order = 4
)

// Colour
type Colour struct {
	R, G, B uint32
	Note    uint8
}

var Notes = []Colour{
	// red
	{0xffff, 0, 0, 62},
	// orange
	{0xffff, 0xa5a5, 0, 64},
	// yellow
	{0xffff, 0xffff, 0, 65},
	// green
	{0, 0xffff, 0, 67},
	// blue
	{0, 0, 0xffff, 69},
	// indigo
	{0x4b4b, 0, 0x8282, 71},
	// violet
	{0x7f00, 0, 0xffff, 60},
}

// State is the state of the markov model
type State [Order]byte

func (s State) Next(next byte) State {
	for i := range s[:len(s)-1] {
		s[i] = s[i+1]
	}
	s[len(s)-1] = next
	return s
}

var (
	// FlagIterations number of iterations
	FlagIterations = flag.Int("i", 8, "number of iterations")
	// FlagMarkov markov mode
	FlagMarkov = flag.Bool("markov", false, "image mode")
	// FlagSmith
	FlagSmith = flag.Bool("smith", false, "smith mode")
)

// MarkovMode is the image mode
func MarkovMode() {
	rng := rand.New(rand.NewSource(1))
	input, err := os.Open("images/image02.png")
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
		Note [7]float64
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
			sum := 0.0
			for _, value := range colors {
				sum += value
			}
			for i, value := range colors {
				entries[index].Note[i] = value / sum
			}
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
	markov := make(map[State][]Entry, 7)
	var learn func(entries []Entry, state State, depth, max int)
	learn = func(entries []Entry, state State, depth, max int) {
		if depth > max || len(entries) == 0 {
			return
		}
		vectors := make([][]float64, len(entries))
		for i := range entries {
			vectors[i] = entries[i].DCT[:]
		}
		clusters, _, err := kmeans.Kmeans(1, vectors, 7, kmeans.SquaredEuclideanDistance, -1)
		if err != nil {
			panic(err)
		}
		for i := range entries {
			state[depth] = byte(clusters[i])
			markov[state] = append(markov[state], entries[i])
		}
		for i := range 7 {
			state[depth] = byte(i)
			learn(markov[state], state, depth+1, max)
		}
	}
	for i := range markov {
		sum := 0.0
		for j := range markov[i] {
			sum += markov[i][j].Rank
		}
		for j := range markov[i] {
			markov[i][j].Rank /= sum
		}
	}
	table := make(map[uint8]int)
	for i := range Notes {
		table[Notes[i].Note] = i
	}
	learn(entries, State{}, 0, Order-1)

	err = writer.WriteSMF("notes.mid", 1, func(wr *writer.SMF) error {
		state := State{}
		for range 33 {
			var entries []Entry
			{
				state, i := state, 1
				for entries == nil {
					entries = markov[state]
					state[len(state)-i] = 0
					i++
				}
			}
			total, selected, index := 0.0, rng.Float64(), 0
			for j := range entries {
				total += entries[j].Rank
				if selected < total {
					index = j
					break
				}
			}
			total, selected, color := 0.0, rng.Float64(), 0
			for j, value := range entries[index].Note {
				total += value
				if selected < total {
					color = j
					break
				}
			}

			wr.SetChannel(0)
			writer.NoteOn(wr, Notes[color].Note, 100)
			wr.SetDelta(120)
			writer.NoteOff(wr, Notes[color].Note)
			wr.SetDelta(240)
			state = state.Next(byte(color))
		}

		writer.EndOfTrack(wr)

		return nil
	})
	if err != nil {
		panic(err)
	}
}

// SmithMode is the smith mode
func SmithMode() {
	rng := rand.New(rand.NewSource(1))
	var u [Size][Size]byte
	rank := func() ([]float64, float64) {
		g := pagerank.NewGraph(Size, rng)
		for i := range u {
			for j := range u {
				if j > i {
					break
				}
				if u[i][j] == 1 && u[j][i] == 1 {
					g.Link(uint32(i), uint32(j), 1)
					g.Link(uint32(j), uint32(i), 1)
				}
			}
		}
		ranks := make([]float64, Size)
		g.Rank(.85, 0.0000001, func(node int, rank float64) {
			ranks[node] = rank
		})
		sum := 0.0
		for _, rank := range ranks {
			sum += rank
		}
		avg := sum / float64(len(ranks))
		v := 0.0
		for _, rank := range ranks {
			diff := rank - avg
			v += diff * diff
		}
		v /= float64(len(ranks))
		return ranks, v
	}
	indexes := rng.Perm(Size)
	for _, i := range indexes {
		count := 0
		for _, value := range u[i] {
			if value != 0 {
				count++
			}
		}
		perm := rng.Perm(Size)
		count = Size/2 - count
		for j, value := range perm {
			if value != 0 {
				continue
			}
			if count == 0 {
				break
			}
			u[i][j] = 1
			u[j][i] = 1
			count--
		}
	}

	err := writer.WriteSMF("notes.mid", 1, func(wr *writer.SMF) error {
		for range *FlagIterations * 1024 {
			ranks, _ := rank()
		search:
			for {
				a, b := rng.Intn(Size), rng.Intn(Size)

				aa, bb := make([]int, 0, 8), make([]int, 0, 8)
				for i, value := range u[a] {
					if value != 0 {
						aa = append(aa, i)
					}
				}
				for i, value := range u[b] {
					if value != 0 {
						bb = append(bb, i)
					}
				}
				rng.Shuffle(len(aa), func(i, j int) {
					aa[i], aa[j] = aa[j], aa[i]
				})
				rng.Shuffle(len(bb), func(i, j int) {
					bb[i], bb[j] = bb[j], bb[i]
				})

				if u[a][b] == 0 && u[b][a] == 0 {
					if len(aa) >= 5 {
						u[a][aa[0]] = 0
						u[aa[0]][a] = 0
					}
					if len(bb) >= 5 {
						u[b][bb[0]] = 0
						u[bb[0]][b] = 0
					}
					u[a][b] = 1
					u[b][a] = 1
					r, _ := rank()
					if r[a] < ranks[a] || r[b] < ranks[b] {
						u[a][b] = 0
						u[b][a] = 0
						if len(aa) >= 5 {
							u[a][aa[0]] = 1
							u[aa[0]][a] = 1
						}
						if len(bb) >= 5 {
							u[b][bb[0]] = 1
							u[bb[0]][b] = 1
						}
					} else {
						wr.SetChannel(0)
						writer.NoteOn(wr, uint8(bb[0]), 100)
						wr.SetDelta(120)
						writer.NoteOff(wr, uint8(bb[0]))
						wr.SetDelta(240)
						break search
					}
				}
			}
			for i := range u {
				for j := range u {
					if u[i][j] != u[j][i] {
						panic("not symmetric")
					}
				}
			}
			for i := range u {
				fmt.Println(u[i])
			}
			fmt.Println()
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()

	if *FlagMarkov {
		MarkovMode()
		return
	}

	if *FlagSmith {
		SmithMode()
		return
	}

	rng := rand.New(rand.NewSource(1))
	input, err := os.Open("images/image02.png")
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
	const Order = 3
	type Entry struct {
		DCT  [N * N]float64
		Rank [Order]float64
		Note [7]float64
		Link []*Entry
		Dist []float64
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
			sum := 0.0
			for _, value := range colors {
				sum += value
			}
			for i, value := range colors {
				entries[index].Note[i] = value / sum
			}
			ForwardDCT(&g, &entries[index].DCT)
			if r > 0 && c > 0 {
				entries[index].Link = append(entries[index].Link, &entries[(r-1)*(width/N)+c-1])
			}
			if r > 0 {
				entries[index].Link = append(entries[index].Link, &entries[(r-1)*(width/N)+c])
			}
			if r > 0 && c < (width/N)-1 {
				entries[index].Link = append(entries[index].Link, &entries[(r-1)*(width/N)+c+1])
			}
			if c < (width/N)-1 {
				entries[index].Link = append(entries[index].Link, &entries[r*(width/N)+c+1])
			}
			if r < (height/N)-1 && c < (width/N)-1 {
				entries[index].Link = append(entries[index].Link, &entries[(r+1)*(width/N)+c+1])
			}
			if r < (height/N)-1 {
				entries[index].Link = append(entries[index].Link, &entries[(r+1)*(width/N)+c])
			}
			if r < (height/N)-1 && c > 0 {
				entries[index].Link = append(entries[index].Link, &entries[(r+1)*(width/N)+c-1])
			}
			if c > 0 {
				entries[index].Link = append(entries[index].Link, &entries[r*(width/N)+c-1])
			}
			index++
		}
	}
	var u [Order]float64
	err = writer.WriteSMF("notes.mid", 1, func(wr *writer.SMF) error {
		entry := []*Entry{
			&entries[0],
		}
		for range Order - 1 {
			entry = append(entry, &entries[rng.Intn(len(entries))])
		}
		for range 256 * 1024 * 1024 {
			if rng.Float64() < entry[0].Rank[0]/u[0] {
				total, selected, color := 0.0, rng.Float64(), 0
				for j, value := range entry[0].Note {
					total += value
					if selected < total {
						color = j
						break
					}
				}

				wr.SetChannel(0)
				writer.NoteOn(wr, Notes[color].Note, 100)
				wr.SetDelta(120)
				writer.NoteOff(wr, Notes[color].Note)
				wr.SetDelta(240)
			}
			entry[0].Rank[0]++
			u[0]++
			for i, v := range entry[1:] {
				if entry[0] != v {
					break
				}
				entry[i+1].Rank[i+1]++
				u[i+1]++
			}

			for i, v := range entry {
				distribution := v.Dist
				if distribution == nil {
					distribution = make([]float64, 0, 8)
					for _, next := range v.Link {
						distribution = append(distribution, math.Abs(CS(&v.DCT, &next.DCT)))
					}
					sum := 0.0
					for _, value := range distribution {
						sum += value
					}
					for i, value := range distribution {
						distribution[i] = value / sum
					}
					v.Dist = distribution
				}
				total, selected, index := 0.0, rng.Float64(), 0
				for i, value := range distribution {
					total += value
					if selected < total {
						index = i
						break
					}
				}
				entry[i] = v.Link[index]
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	p := plot.New()
	p.Title.Text = "y vs x"
	p.X.Label.Text = "x"
	p.Y.Label.Text = "y"

	points := make(plotter.XYs, 0, 8)
	for i, entry := range entries {
		points = append(points, plotter.XY{X: float64(i), Y: entry.Rank[0] / u[0]})
	}
	scatter, err := plotter.NewScatter(points)
	if err != nil {
		panic(err)
	}
	scatter.GlyphStyle.Radius = vg.Length(1)
	scatter.GlyphStyle.Shape = draw.CircleGlyph{}
	p.Add(scatter)

	err = p.Save(8*vg.Inch, 8*vg.Inch, "plot0.png")
	if err != nil {
		panic(err)
	}

	for i := range u[1:] {
		points := make(plotter.XYs, 0, 8)
		for x, entry := range entries {
			points = append(points, plotter.XY{X: float64(x), Y: entry.Rank[i+1] / u[i+1]})
		}
		scatter, err := plotter.NewScatter(points)
		if err != nil {
			panic(err)
		}
		scatter.GlyphStyle.Radius = vg.Length(1)
		scatter.GlyphStyle.Shape = draw.CircleGlyph{}
		note := Notes[i]
		scatter.GlyphStyle.Color = color.RGBA{
			R: uint8(note.R >> 8),
			G: uint8(note.G >> 8),
			B: uint8(note.B >> 8),
			A: 0xff,
		}
		p.Add(scatter)

		{
			p := plot.New()
			p.Title.Text = "y vs x"
			p.X.Label.Text = "x"
			p.Y.Label.Text = "y"

			scatter, err := plotter.NewScatter(points)
			if err != nil {
				panic(err)
			}
			scatter.GlyphStyle.Radius = vg.Length(1)
			scatter.GlyphStyle.Shape = draw.CircleGlyph{}
			p.Add(scatter)

			err = p.Save(8*vg.Inch, 8*vg.Inch, fmt.Sprintf("plot%d.png", i+1))
			if err != nil {
				panic(err)
			}
		}

		count, sum, sum2 := 0.0, 0.0, 0.0
		for _, entry := range entries {
			/*if entry.Rank == 0 || entry.Meta == 0 {
				continue
			}*/
			//fmt.Println(entry.Rank/u, entry.Meta/u2)
			count++
			sum += entry.Rank[0] / u[0]
			sum2 += entry.Rank[i+1] / u[i+1]
		}
		avg := sum / count
		avg2 := sum2 / count
		stddev, stddev2 := 0.0, 0.0
		for _, entry := range entries {
			/*if entry.Rank == 0 || entry.Meta == 0 {
				continue
			}*/
			diff := avg - entry.Rank[0]/u[0]
			stddev += diff * diff
			diff2 := avg2 - entry.Rank[i+1]/u[i+1]
			stddev2 += diff2 * diff2
		}
		stddev /= count
		stddev2 /= count
		stddev = math.Sqrt(stddev)
		stddev2 = math.Sqrt(stddev2)
		corr := 0.0
		for _, entry := range entries {
			/*if entry.Rank == 0 || entry.Meta == 0 {
				continue
			}*/
			corr += (entry.Rank[0]/u[0] - avg) * (entry.Rank[i+1]/u[i+1] - avg2)
		}
		corr /= count
		corr /= stddev * stddev2
		fmt.Println(avg2, stddev2, corr)
	}

	err = p.Save(8*vg.Inch, 8*vg.Inch, "plot.png")
	if err != nil {
		panic(err)
	}
}
