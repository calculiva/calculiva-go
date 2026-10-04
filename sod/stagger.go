package sod

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/calculiva/calculiva-go/internal/num"
)

// The staggered row layout used by LayRect: horizontal rows, unbalanced edge
// row, no gap, no kerf, factory-end pairing of start and end offcuts.

// maxPieces is the upper bound on individually placed pieces in one layout.
const maxPieces = 2500

type role int

const (
	roleFull role = iota
	roleIsolated
	roleStart
	roleEnd
)

func (r role) String() string {
	switch r {
	case roleIsolated:
		return "isolated"
	case roleStart:
		return "start"
	case roleEnd:
		return "end"
	default:
		return "full"
	}
}

type plank struct {
	id     string
	row    string
	length float64
	width  float64
	role   role
	cut    bool
}

type staggeredRow struct {
	id     string
	width  float64
	across float64
}

type source struct {
	id     string
	pieces []string
	pair   bool
}

type staggeredLayout struct {
	rows    []staggeredRow
	pieces  []plank
	sources []source
}

func shortest(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func positive(v float64, key, label string, min, max float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return num.NewError(fmt.Sprintf("Enter %s between %s and %s mm.", label, shortest(min), shortest(max)), key)
	}
	return nil
}

func rowWidths(span, width float64) ([]float64, error) {
	if !(span > 0.0 && width > 0.0) {
		return nil, num.NewError("The usable width must be positive.", "roomW")
	}
	n := num.CeilULP(span / width)
	if n == 1.0 {
		return []float64{span}, nil
	}
	last := span - float64((n-1.0)*width)
	v := make([]float64, 0, int(n))
	for i := 0; i < int(n)-1; i++ {
		v = append(v, width)
	}
	return append(v, last), nil
}

// pairCuts pairs start and end offcuts that fit one piece. Each pair uses
// opposite factory ends; no same-end pairing or face flipping.
func pairCuts(pieces []plank, length float64) []source {
	sources := []source{}
	assigned := make([]bool, len(pieces))
	add := func(idx ...int) {
		ids := make([]string, 0, len(idx))
		for _, i := range idx {
			ids = append(ids, pieces[i].id)
			assigned[i] = true
		}
		sources = append(sources, source{id: fmt.Sprintf("B%03d", len(sources)+1), pieces: ids, pair: len(idx) == 2})
	}
	starts := []int{}
	for i, p := range pieces {
		if p.role == roleStart {
			starts = append(starts, i)
		}
	}
	sort.SliceStable(starts, func(x, y int) bool { return pieces[starts[x]].length > pieces[starts[y]].length })
	for _, a := range starts {
		best := -1
		for i, p := range pieces {
			if p.role == roleEnd && !assigned[i] && p.length+pieces[a].length <= length+num.Eps {
				if best < 0 || !(pieces[best].length >= p.length) {
					best = i
				}
			}
		}
		if best >= 0 {
			add(best, a)
		}
	}
	for i := range pieces {
		if !assigned[i] {
			add(i)
		}
	}
	return sources
}

// staggered lays horizontal staggered rows over roomW x roomH, the first row
// starting with a full plank, offset of a plank per row, start and end offcuts
// paired when they fit one plank.
func staggered(roomW, roomH, plankW, plankL, offset, maxRoom, maxPlankW float64) (*staggeredLayout, error) {
	if err := positive(roomW, "roomW", "the room width", 10.0, maxRoom); err != nil {
		return nil, err
	}
	if err := positive(roomH, "roomH", "the room length", 10.0, maxRoom); err != nil {
		return nil, err
	}
	if err := positive(plankW, "plankW", "the plank width", 10.0, maxPlankW); err != nil {
		return nil, err
	}
	if err := positive(plankL, "plankL", "the plank length", 10.0, 6000.0); err != nil {
		return nil, err
	}
	first := plankL
	span, run := roomH, roomW
	widths, err := rowWidths(span, plankW)
	if err != nil {
		return nil, err
	}
	if len(widths) > maxPieces {
		return nil, num.NewError("Use a larger plank or a smaller field.", "plankW")
	}
	rows := make([]staggeredRow, 0, len(widths))
	pieces := []plank{}
	across := 0.0
	for index, width := range widths {
		phase := first + float64(float64(index)*offset*plankL)
		start := math.Mod(math.Mod(phase, plankL)+plankL, plankL)
		if start < num.Eps || plankL-start < num.Eps {
			start = plankL
		}
		rowID := fmt.Sprintf("R%02d", index+1)
		pos := 0.0
		for pos < run-num.Eps {
			if len(pieces) >= maxPieces {
				return nil, num.NewError("This field exceeds 2,500 placed planks.", "roomW")
			}
			proposed := plankL
			if pos == 0.0 {
				proposed = start
			}
			length := math.Min(proposed, run-pos)
			atStart := pos < num.Eps
			atEnd := pos+length >= run-num.Eps
			var r role
			switch {
			case length >= plankL-num.Eps:
				r = roleFull
			case atStart && atEnd:
				r = roleIsolated
			case atStart:
				r = roleStart
			default:
				r = roleEnd
			}
			pieces = append(pieces, plank{
				id:     fmt.Sprintf("P%03d", len(pieces)+1),
				row:    rowID,
				length: length,
				width:  width,
				role:   r,
				cut:    length < plankL-num.Eps || width < plankW-num.Eps,
			})
			pos += length
		}
		rows = append(rows, staggeredRow{id: rowID, width: width, across: across})
		across += width
	}
	return &staggeredLayout{rows: rows, pieces: pieces, sources: pairCuts(pieces, plankL)}, nil
}
