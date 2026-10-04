// Package sod estimates sod by area: staggered layout of rectangles with offcut
// reuse, areas by square footage with an allowance, plugs (UF/IFAS Table 3),
// soil preparation, pallets, cost and weight.
//
// Lengths are millimetres. No format, pallet, allowance, depth or weight lives
// in the code: they come from [Rules], built by [NewRules] from the constants
// below, except the allowances, which the user enters. The results are planning quantities: the grower's pallet, the
// publishers quoted and the installer on site govern.
//
// Calculator page: https://calculiva.com/landscaping/sod-calculator/
package sod

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/internal/num"
)

const eps = 1e-7

// SqFt is one square foot in mm².
const SqFt = calculiva.Foot * calculiva.Foot

const yard = 3.0 * calculiva.Foot

// SqYd is one square yard in mm².
const SqYd = yard * yard

// CuFt is one cubic foot in mm³.
const CuFt = SqFt * calculiva.Foot

// CuYd is one cubic yard in mm³.
const CuYd = yard * yard * yard

const (
	sqM       float64 = 1e6
	rectMax           = 300.0 * calculiva.Foot
	pieceWMin         = 4.0 * calculiva.Inch
	pieceWMax         = 48.0 * calculiva.Inch
	pieceLMin         = 12.0 * calculiva.Inch
	pieceLMax         = 120.0 * calculiva.Inch
)

// FormatRow is a sod format as surveyed (inches and square feet as published).
// A nil slice means the figure is not published for the format.
type FormatRow struct {
	// ID is the format id.
	ID string
	// Name is the display name.
	Name string
	// WidthIn is the piece width, inches.
	WidthIn float64
	// LengthIn is the piece length, inches.
	LengthIn float64
	// StatedSqFt is the square feet per piece as stated by the grower.
	StatedSqFt float64
	// PalletsStatedSqFt are the pallet coverages as stated (slabs), square feet.
	PalletsStatedSqFt []float64
	// PiecesPerPallet are the pieces per pallet as stated.
	PiecesPerPallet []float64
	// PalletsSoldAsSqFt are the pallet coverages as sold, square feet.
	PalletsSoldAsSqFt []float64
}

// Formats are the four surveyed sod formats.
//
// Data sheet sod-format-observations (formats), source
// https://sodsolutions.com/sod-university/square-feet-per-pallet/, verified 2026-09-25,
// market survey expiring 2027-03-25.
var Formats = []FormatRow{
	{
		ID:                "slab-16x24",
		Name:              "Slab, 16 × 24 in",
		WidthIn:           16.0,
		LengthIn:          24.0,
		StatedSqFt:        2.66,
		PalletsStatedSqFt: []float64{399.0, 452.0, 503.0},
		PalletsSoldAsSqFt: []float64{400.0, 450.0, 500.0},
	},
	{
		ID:                "mini-roll-18x40",
		Name:              "Mini hand roll, 18 × 40 in",
		WidthIn:           18.0,
		LengthIn:          40.0,
		StatedSqFt:        5.0,
		PiecesPerPallet:   []float64{80.0, 90.0, 100.0},
		PalletsSoldAsSqFt: []float64{400.0, 450.0, 500.0},
	},
	{
		ID:         "roll-24x60",
		Name:       "Roll, 24 × 60 in",
		WidthIn:    24.0,
		LengthIn:   60.0,
		StatedSqFt: 10.0,
	},
	{
		ID:         "roll-18x80",
		Name:       "Roll, 18 × 80 in",
		WidthIn:    18.0,
		LengthIn:   80.0,
		StatedSqFt: 10.0,
	},
}

// SlabPiecesPerStatedPallet are the slabs on the three stated slab pallets
// (399, 452 and 503 sq ft at 2.66 sq ft).
//
// Data sheet sod-format-observations (derived.slab_pieces_per_stated_pallet), source
// https://sodsolutions.com/sod-university/square-feet-per-pallet/, verified 2026-09-25,
// market survey expiring 2027-03-25.
var SlabPiecesPerStatedPallet = []float64{150.0, 170.0, 189.0}

// PalletWeightLb is the weight of one pallet of sod, pounds (a range).
//
// Data sheet sod-format-observations (source sodsolutions,
// https://sodsolutions.com/sod-university/square-feet-per-pallet/), verified 2026-09-25,
// market survey expiring 2027-03-25.
var PalletWeightLb = [2]float64{1500.0, 3000.0}

// PSUTopsoilSettledIn is the settled topsoil under new sod, inches (Penn State).
//
// Data sheet sod-installation-guidance (publisher psu), source
// https://extension.psu.edu/lawn-establishment/, verified 2026-09-25.
var PSUTopsoilSettledIn = [2]float64{4.0, 6.0}

// PSUOrganicLayerIn is the organic matter layer to incorporate, inches
// (Penn State).
//
// Data sheet sod-installation-guidance (publisher psu), source
// https://extension.psu.edu/lawn-establishment/, verified 2026-09-25.
var PSUOrganicLayerIn = [2]float64{1.0, 2.0}

// PSUOrganicLayerMaxSinglePassIn is the thickest organic layer to incorporate
// in a single pass, inches (Penn State).
//
// Data sheet sod-installation-guidance (publisher psu), source
// https://extension.psu.edu/lawn-establishment/, verified 2026-09-25.
const PSUOrganicLayerMaxSinglePassIn float64 = 2.0

// NCSUOrganicCuYdPer1000SqFt is the organic matter rate, cubic yards per
// 1,000 sq ft (NC State).
//
// Data sheet sod-installation-guidance (publisher ncsu, https://content.ces.ncsu.edu/carolina-lawns),
// sheet source https://extension.psu.edu/lawn-establishment/, verified 2026-09-25.
var NCSUOrganicCuYdPer1000SqFt = [2]float64{1.0, 2.0}

// PlantingRow is a row of UF/IFAS LH013 Table 3.
type PlantingRow struct {
	// Species is the species as printed.
	Species string
	// Method is the planting method as printed.
	Method string
	// SpacingIn is the spacing, inches.
	SpacingIn float64
	// SodSqFtPer1000 is the square feet of sod per 1,000 sq ft of lawn (a range).
	SodSqFtPer1000 [2]float64
}

// IFASTable3 is UF/IFAS LH013 Table 3: plugs and sprigs by species.
//
// Data sheet sod-plugging-ifas (table3), source https://ask.ifas.ufl.edu/publication/LH013,
// verified 2026-09-25.
var IFASTable3 = []PlantingRow{
	{Species: "St. Augustinegrass", Method: "2-inch plugs", SpacingIn: 12.0, SodSqFtPer1000: [2]float64{30.0, 50.0}},
	{Species: "St. Augustinegrass", Method: "sprigs", SpacingIn: 12.0, SodSqFtPer1000: [2]float64{10.0, 15.0}},
	{Species: "Centipedegrass", Method: "2-inch plugs", SpacingIn: 6.0, SodSqFtPer1000: [2]float64{100.0, 150.0}},
	{Species: "Centipedegrass", Method: "sprigs", SpacingIn: 6.0, SodSqFtPer1000: [2]float64{30.0, 50.0}},
	{Species: "Zoysiagrass", Method: "2-inch plugs", SpacingIn: 6.0, SodSqFtPer1000: [2]float64{100.0, 150.0}},
	{Species: "Zoysiagrass", Method: "sprigs", SpacingIn: 6.0, SodSqFtPer1000: [2]float64{8.0, 15.0}},
	{Species: "Bermudagrass", Method: "2-inch plugs", SpacingIn: 12.0, SodSqFtPer1000: [2]float64{30.0, 50.0}},
	{Species: "Bermudagrass", Method: "sprigs", SpacingIn: 12.0, SodSqFtPer1000: [2]float64{2.0, 5.0}},
}

// TwoInchPlugsPerSqYd is the number of two-inch plugs cut from one square yard
// of sod.
//
// Data sheet sod-plugging-ifas (conversions.two_inch_plugs_per_sqyd), source
// https://ask.ifas.ufl.edu/publication/LH013, verified 2026-09-25.
const TwoInchPlugsPerSqYd float64 = 324.0

// PlugSizesIn are the plug sizes offered, inches: UF/IFAS LH013 "2-to-4-inch"
// plugs.
//
// Data sheet sod-plugging-ifas, source https://ask.ifas.ufl.edu/publication/LH013, verified 2026-09-25.
var PlugSizesIn = []float64{2.0, 3.0, 4.0}

// Format is a sod format in millimetres, with its pallets. A nil slice means
// the figure is not known for the format.
type Format struct {
	// ID is the format id.
	ID string
	// Name is the display name.
	Name string
	// WidthMM is the piece width, mm.
	WidthMM float64
	// LengthMM is the piece length, mm.
	LengthMM float64
	// StatedSqFt is the stated square feet per piece.
	StatedSqFt float64
	// PalletPieces are the pieces per pallet, when known.
	PalletPieces []float64
	// PalletSqFt are the pallet coverages, square feet, when known.
	PalletSqFt []float64
	// SoldAsSqFt are the pallet coverages as sold, when published.
	SoldAsSqFt []float64
}

// Species is a plugging species.
type Species struct {
	// ID is the slug of the species name.
	ID string
	// Species is the species as printed.
	Species string
	// SpacingMM is the table spacing, mm.
	SpacingMM float64
	// PlugIn is the plug size of the table row, inches.
	PlugIn float64
	// SodSqFtPer1000 is the square feet of sod per 1,000 sq ft.
	SodSqFtPer1000 [2]float64
}

// Rules are the rules of the sod calculator.
type Rules struct {
	// Formats are the formats.
	Formats []Format
	// Allowance is the default allowance, percent: an entry of the user, 0
	// unless set.
	Allowance float64
	// AllowanceComplex is the allowance for complex shapes, percent: an entry
	// of the user, 0 unless set.
	AllowanceComplex float64
	// PalletLb is the pallet weight range, lb.
	PalletLb [2]float64
	// TopsoilMM is the settled topsoil range, mm.
	TopsoilMM [2]float64
	// OrganicLayerMM is the organic layer range, mm.
	OrganicLayerMM [2]float64
	// OrganicLayerMaxMM is the thickest single-pass organic layer, mm.
	OrganicLayerMaxMM float64
	// OrganicRate is the organic rate range, cu yd per 1,000 sq ft.
	OrganicRate [2]float64
	// Species are the plugging species.
	Species []Species
	// PlugsPerSqYd2In is the number of two-inch plugs per square yard of sod.
	PlugsPerSqYd2In float64
	// PlugSizes are the plug sizes offered, inches.
	PlugSizes []float64
}

func slug(s string) string {
	var out strings.Builder
	dash := false
	for _, ch := range strings.ToLower(s) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			if dash && out.Len() > 0 {
				out.WriteByte('-')
			}
			dash = false
			out.WriteRune(ch)
		} else {
			dash = true
		}
	}
	return out.String()
}

func leadingNumber(s string) float64 {
	end := strings.IndexFunc(s, func(c rune) bool { return !((c >= '0' && c <= '9') || c == '.') })
	if end < 0 {
		end = len(s)
	}
	v, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return math.NaN()
	}
	return v
}

func clone(v []float64) []float64 {
	if v == nil {
		return nil
	}
	return append([]float64{}, v...)
}

// NewRules builds the sod rules from the sourced constants of this package.
// Allowance and AllowanceComplex are left at 0 for the user to set.
func NewRules() Rules {
	formats := make([]Format, 0, len(Formats))
	for _, f := range Formats {
		palletPieces := f.PiecesPerPallet
		if f.PalletsStatedSqFt != nil {
			palletPieces = SlabPiecesPerStatedPallet
		}
		palletSqFt := f.PalletsStatedSqFt
		if palletSqFt == nil {
			palletSqFt = f.PalletsSoldAsSqFt
		}
		formats = append(formats, Format{
			ID:           f.ID,
			Name:         f.Name,
			WidthMM:      f.WidthIn * calculiva.Inch,
			LengthMM:     f.LengthIn * calculiva.Inch,
			StatedSqFt:   f.StatedSqFt,
			PalletPieces: clone(palletPieces),
			PalletSqFt:   clone(palletSqFt),
			SoldAsSqFt:   clone(f.PalletsSoldAsSqFt),
		})
	}
	species := []Species{}
	for _, r := range IFASTable3 {
		if !strings.Contains(r.Method, "plug") {
			continue
		}
		species = append(species, Species{
			ID:             slug(r.Species),
			Species:        r.Species,
			SpacingMM:      r.SpacingIn * calculiva.Inch,
			PlugIn:         leadingNumber(r.Method),
			SodSqFtPer1000: r.SodSqFtPer1000,
		})
	}
	return Rules{
		Formats:           formats,
		PalletLb:          PalletWeightLb,
		TopsoilMM:         [2]float64{PSUTopsoilSettledIn[0] * calculiva.Inch, PSUTopsoilSettledIn[1] * calculiva.Inch},
		OrganicLayerMM:    [2]float64{PSUOrganicLayerIn[0] * calculiva.Inch, PSUOrganicLayerIn[1] * calculiva.Inch},
		OrganicLayerMaxMM: PSUOrganicLayerMaxSinglePassIn * calculiva.Inch,
		OrganicRate:       NCSUOrganicCuYdPer1000SqFt,
		Species:           species,
		PlugsPerSqYd2In:   TwoInchPlugsPerSqYd,
		PlugSizes:         clone(PlugSizesIn),
	}
}

// Piece is one placed piece of a rectangle layout.
type Piece struct {
	// ID is P001, P002 and so on.
	ID string
	// Row is the row id.
	Row string
	// Length is the laid length, mm.
	Length float64
	// Width is the laid width, mm.
	Width float64
	// Role is full, isolated, start or end.
	Role string
	// Cut tells whether the piece is cut.
	Cut bool
	// Buy is the bought piece it comes from (B001 or strip source S001).
	Buy string
	// Shared tells whether that bought piece is shared with another placed
	// piece.
	Shared bool
}

// Row is a row of a rectangle layout.
type Row struct {
	// ID is R01, R02 and so on.
	ID string
	// Width is the row width, mm.
	Width float64
	// Across is the offset of the row across the area, mm.
	Across float64
}

// RectLayout is the layout of one rectangle.
type RectLayout struct {
	// Pieces are the placed pieces.
	Pieces []Piece
	// Rows are the rows.
	Rows []Row
	// Installed is the number of placed pieces.
	Installed int
	// Cut is the number of cut pieces.
	Cut int
	// Full is the number of uncut pieces.
	Full int
	// Pairs is the number of start and end offcuts paired in one piece.
	Pairs int
	// Strips is the number of narrow full-length strips of the last row.
	Strips int
	// StripBought is the number of pieces bought for those strips.
	StripBought int
	// Bought is the number of pieces bought.
	Bought int
}

// LayRect lays a rectangle: staggered rows at half a piece, then the narrow
// full-length strips of the last row grouped floor(piece width / strip width)
// per bought piece (UF/IFAS: offcuts fill the joints; a strip taken from a
// piece leaves a strip).
func LayRect(l, w, pieceW, pieceL float64) (*RectLayout, error) {
	fp, err := staggered(l, w, pieceW, pieceL, 0.5, rectMax, pieceWMax)
	if err != nil {
		return nil, err
	}
	index := make(map[string]int, len(fp.pieces))
	for i, p := range fp.pieces {
		index[p.id] = i
	}
	buy := make([]string, len(fp.pieces))
	shared := make([]bool, len(fp.pieces))
	type group struct {
		key   string
		width float64
		ids   []int
	}
	groups := []group{}
	kept := 0
	for _, s := range fp.sources {
		i := index[s.pieces[0]]
		p := fp.pieces[i]
		if !s.pair && p.role == roleFull && p.width < pieceW-eps {
			k := fmt.Sprintf("%.3f", p.width)
			found := false
			for g := range groups {
				if groups[g].key == k {
					groups[g].ids = append(groups[g].ids, i)
					found = true
					break
				}
			}
			if !found {
				groups = append(groups, group{key: k, width: p.width, ids: []int{i}})
			}
			continue
		}
		kept++
		for _, id := range s.pieces {
			j := index[id]
			buy[j] = s.id
			shared[j] = s.pair
		}
	}
	strips, stripBought, n := 0, 0, 0
	for _, g := range groups {
		per := int(math.Max(num.FloorRel(pieceW/g.width), 1.0))
		bought := int(num.CeilRel(float64(len(g.ids)) / float64(per)))
		strips += len(g.ids)
		stripBought += bought
		for k, i := range g.ids {
			buy[i] = fmt.Sprintf("S%03d", n+k/per+1)
			shared[i] = per > 1 && len(g.ids) > 1
		}
		n += bought
	}
	pieces := make([]Piece, 0, len(fp.pieces))
	cut := 0
	for i, p := range fp.pieces {
		pieces = append(pieces, Piece{
			ID:     p.id,
			Row:    p.row,
			Length: p.length,
			Width:  p.width,
			Role:   p.role.String(),
			Cut:    p.cut,
			Buy:    buy[i],
			Shared: shared[i],
		})
		if p.cut {
			cut++
		}
	}
	pairs := 0
	for _, s := range fp.sources {
		if s.pair {
			pairs++
		}
	}
	rows := make([]Row, 0, len(fp.rows))
	for _, r := range fp.rows {
		rows = append(rows, Row{ID: r.id, Width: r.width, Across: r.across})
	}
	return &RectLayout{
		Pieces:      pieces,
		Rows:        rows,
		Installed:   len(pieces),
		Cut:         cut,
		Full:        len(pieces) - cut,
		Pairs:       pairs,
		Strips:      strips,
		StripBought: stripBought,
		Bought:      kept + stripBought,
	}, nil
}

// Shape is the shape of a sod area, lengths in mm: [Rect], [Circle],
// [Triangle] or [KnownArea].
type Shape interface {
	isShape()
}

// Rect is a rectangle (laid out piece by piece).
type Rect struct {
	// L is the length.
	L float64
	// W is the width.
	W float64
	// Count is the number of copies (1 to 20).
	Count float64
}

// Circle is a circle of radius R.
type Circle struct {
	// R is the radius.
	R float64
	// Count is the number of copies.
	Count float64
}

// Triangle is a triangle.
type Triangle struct {
	// B is the base.
	B float64
	// H is the height.
	H float64
	// Count is the number of copies.
	Count float64
}

// KnownArea is a known area, mm².
type KnownArea struct {
	// Area is the area in mm².
	Area float64
}

func (Rect) isShape()      {}
func (Circle) isShape()    {}
func (Triangle) isShape()  {}
func (KnownArea) isShape() {}

// Area is one sod area.
type Area struct {
	// ID is the area id; it prefixes error fields.
	ID string
	// Shape is the area's shape.
	Shape Shape
}

// PalletMode tells how a pallet is sold.
type PalletMode int

const (
	// PalletByPieces is a pallet sold by piece count.
	PalletByPieces PalletMode = iota
	// PalletBySqFt is a pallet sold by square feet.
	PalletBySqFt
)

// Method is solid sod or plugs.
type Method int

const (
	// MethodSolid is solid sod.
	MethodSolid Method = iota
	// MethodPlugs is plugs cut from sod.
	MethodPlugs
)

// Organic tells how organic matter is given.
type Organic int

const (
	// OrganicLayer gives it as a layer depth.
	OrganicLayer Organic = iota
	// OrganicRate gives it as cubic yards per 1,000 sq ft.
	OrganicRate
	// OrganicNone means none.
	OrganicNone
)

// PriceMode tells how the price is quoted.
type PriceMode int

const (
	// PricePerPallet is a price per pallet.
	PricePerPallet PriceMode = iota
	// PricePerPiece is a price per piece.
	PricePerPiece
	// PricePerSqFt is a price per square foot.
	PricePerSqFt
)

// Input is the input of [NewPlan]. Build it with [NewInput] to get the
// calculator defaults.
type Input struct {
	// Areas holds one to eight areas.
	Areas []Area
	// Format is a format id of the rules, or "custom".
	Format string
	// PieceW is the custom piece width, mm.
	PieceW float64
	// PieceL is the custom piece length, mm.
	PieceL float64
	// PalletMode is the pallet sale mode.
	PalletMode PalletMode
	// PalletPieces is the number of pieces per pallet (whole number).
	PalletPieces float64
	// PalletSqFt is the square feet per pallet.
	PalletSqFt float64
	// Allowance is the allowance on areas without a layout, percent.
	Allowance float64
	// Spare is the spare on laid-out rectangles, percent.
	Spare float64
	// Method is solid sod or plugs.
	Method Method
	// Species is a species id of the rules, or "custom".
	Species string
	// Spacing is the plug spacing, mm.
	Spacing float64
	// PlugSize is the plug size, inches.
	PlugSize float64
	// Topsoil is the topsoil depth, mm.
	Topsoil float64
	// Organic is the organic matter mode.
	Organic Organic
	// OrganicIn is the organic layer, mm.
	OrganicIn float64
	// OrganicRate is the organic rate, cu yd per 1,000 sq ft.
	OrganicRate float64
	// PriceMode is the price mode.
	PriceMode PriceMode
	// Price is the price.
	Price float64
	// Delivery is the delivery charge.
	Delivery float64
	// Payload is the vehicle payload, lb, 0 for none.
	Payload float64
}

// NewInput returns an input with the calculator defaults taken from the rules
// (first format, rule allowance, first species, lower topsoil and organic
// values). PalletPieces starts at 0 and must be set.
func NewInput(areas []Area, rules Rules) Input {
	sp := rules.Species[0]
	return Input{
		Areas:       areas,
		Format:      rules.Formats[0].ID,
		PalletMode:  PalletByPieces,
		Allowance:   rules.Allowance,
		Method:      MethodSolid,
		Species:     sp.ID,
		Spacing:     sp.SpacingMM,
		PlugSize:    sp.PlugIn,
		Topsoil:     rules.TopsoilMM[0],
		Organic:     OrganicLayer,
		OrganicIn:   rules.OrganicLayerMM[0],
		OrganicRate: rules.OrganicRate[0],
		PriceMode:   PricePerPallet,
	}
}

// Mode tells how an area was counted.
type Mode int

const (
	// ModePlugs means plugs: counted over the whole lawn.
	ModePlugs Mode = iota
	// ModeLayout means laid out piece by piece.
	ModeLayout
	// ModeArea means counted by area with the allowance.
	ModeArea
	// ModeNoLayout means a rectangle too large to lay out, counted by area.
	ModeNoLayout
)

// AreaPlan is one area's result.
type AreaPlan struct {
	// ID is the area id.
	ID string
	// Shape is the shape.
	Shape Shape
	// Count is the number of copies.
	Count float64
	// AreaMM2 is the area of one copy, mm².
	AreaMM2 float64
	// SqFt is the area of all copies, sq ft.
	SqFt float64
	// Mode tells how it was counted.
	Mode Mode
	// Layout is the layout, for laid-out rectangles; nil otherwise.
	Layout *RectLayout
	// Bought is the number of pieces bought.
	Bought float64
	// ExtraPct is the extra over the area, percent; nil for plugs.
	ExtraPct *float64
}

// Totals are the totals.
type Totals struct {
	// AreaMM2 is the lawn area, mm².
	AreaMM2 float64
	// SqFt is the lawn area, sq ft.
	SqFt float64
	// LayoutBought is the number of pieces for laid-out rectangles.
	LayoutBought float64
	// AreaBought is the number of pieces for areas counted by area.
	AreaBought float64
	// Spare is the number of spare pieces.
	Spare float64
	// Pieces is the number of pieces to buy.
	Pieces float64
	// Flat is the flat count at the rule allowance, for comparison.
	Flat float64
}

// Pallets is the pallet count.
type Pallets struct {
	// PerPallet is the number of pieces per pallet.
	PerPallet float64
	// Full is the number of full pallets.
	Full float64
	// Loose is the number of loose pieces over the full pallets.
	Loose float64
	// PalletsUp is the number of pallets rounded up.
	PalletsUp float64
	// Leftover is the number of pieces left on the last pallet.
	Leftover float64
}

// PlugPlan is the plug result.
type PlugPlan struct {
	// Species is the species, nil when custom.
	Species *Species
	// Count is the number of plugs.
	Count float64
	// SodSqFt is the sod consumed, sq ft.
	SodSqFt float64
	// SodSqYd is the sod consumed, sq yd.
	SodSqYd float64
	// PerSqYd is the number of plugs per square yard of sod.
	PerSqYd float64
	// Pieces is the number of sod pieces to buy.
	Pieces float64
	// TableSqFt is the table sod range for this lawn, sq ft; nil when the
	// table has no row for the species and plug size.
	TableSqFt *[2]float64
	// TablePieces is the table sod range, pieces; nil with TableSqFt.
	TablePieces *[2]float64
}

// Prep is the soil preparation. At most one of the two equivalents is set: it
// is the other publisher's expression of the organic matter.
type Prep struct {
	// TopsoilMM3 is the topsoil, mm³.
	TopsoilMM3 float64
	// TopsoilCuFt is the topsoil, cu ft.
	TopsoilCuFt float64
	// TopsoilCuYd is the topsoil, cu yd.
	TopsoilCuYd float64
	// OrganicMM3 is the organic matter, mm³.
	OrganicMM3 float64
	// OrganicCuFt is the organic matter, cu ft.
	OrganicCuFt float64
	// OrganicCuYd is the organic matter, cu yd.
	OrganicCuYd float64
	// EquivalentRate is the equivalent rate, cu yd per 1,000 sq ft, when the
	// organic matter was given as a layer.
	EquivalentRate *float64
	// EquivalentLayerMM is the equivalent layer, mm, when the organic matter
	// was given as a rate.
	EquivalentLayerMM *float64
}

// Cost is the cost. A nil pointer means the value is absent.
type Cost struct {
	// PerPiece is the price per piece, when quoted per piece or per square
	// foot.
	PerPiece *float64
	// Whole is whole pallets plus delivery.
	Whole *float64
	// Mixed is full pallets plus loose pieces plus delivery.
	Mixed *float64
}

// WarningKind is the short kind of a warning, as used on the calculator page.
type WarningKind string

const (
	// NoLayout flags a rectangle too large to lay out.
	NoLayout WarningKind = "no-layout"
	// Spacing flags a spacing that differs from the species' table spacing.
	Spacing WarningKind = "spacing"
	// OrganicThick flags an organic layer thicker than one pass.
	OrganicThick WarningKind = "organic-thick"
	// RateOutside flags a rate outside the published range.
	RateOutside WarningKind = "rate-outside"
	// TopsoilThin flags topsoil thinner than published.
	TopsoilThin WarningKind = "topsoil-thin"
)

// Warning is a planning warning. The fields that do not belong to its kind are
// left at zero.
type Warning struct {
	// Kind is the kind of warning.
	Kind WarningKind
	// Area is the area id (NoLayout).
	Area string
	// Species is the species (Spacing).
	Species string
	// Spacing is the spacing entered, mm (Spacing).
	Spacing float64
	// TableMM is the table spacing, mm (Spacing).
	TableMM float64
	// Layer is the organic layer, mm (OrganicThick).
	Layer float64
	// MaxMM is the maximum, mm (OrganicThick).
	MaxMM float64
	// Rate is the rate (RateOutside).
	Rate float64
	// Range is the published range (RateOutside).
	Range [2]float64
	// Topsoil is the topsoil, mm (TopsoilThin).
	Topsoil float64
	// MinMM is the minimum, mm (TopsoilThin).
	MinMM float64
}

// Plan is the result of [NewPlan].
type Plan struct {
	// PieceW is the piece width, mm.
	PieceW float64
	// PieceL is the piece length, mm.
	PieceL float64
	// PieceMM2 is the piece area, mm².
	PieceMM2 float64
	// PieceSqFt is the piece area, sq ft.
	PieceSqFt float64
	// PerPallet is the number of pieces per pallet.
	PerPallet float64
	// Areas are the areas.
	Areas []AreaPlan
	// Totals are the totals.
	Totals Totals
	// Pallets is the pallet count.
	Pallets Pallets
	// Plug is the plug result, when planting plugs; nil otherwise.
	Plug *PlugPlan
	// Prep is the soil preparation.
	Prep Prep
	// Cost is the cost.
	Cost Cost
	// WeightLb is the weight range of the pallets, lb.
	WeightLb [2]float64
	// Trips is the number of trips for each end of the weight range; nil
	// without a payload.
	Trips *[2]float64
	// Warnings are the warnings.
	Warnings []Warning
}

func tidy(x float64) float64 {
	if math.Abs(x) < 1e-9 {
		return 0.0
	}
	return x
}

func ptr(v float64) *float64 { return &v }

// NewPlan plans sod: pieces per area, pallets, plugs, soil preparation, cost
// and weight.
func NewPlan(c Input, rules Rules) (*Plan, error) {
	if len(c.Areas) == 0 || len(c.Areas) > 8 {
		return nil, num.NewError("Tick one to eight areas.", "areas")
	}
	// Format: one of the surveyed formats, or a piece entered by hand.
	var pieceW, pieceL float64
	known := false
	for _, f := range rules.Formats {
		if f.ID == c.Format {
			pieceW, pieceL, known = f.WidthMM, f.LengthMM, true
			break
		}
	}
	if !known {
		if c.Format != "custom" {
			return nil, num.NewError("Choose a sod format.", "format")
		}
		if err := num.Range(c.PieceW, pieceWMin, pieceWMax, "Enter a piece width from 4 to 48 in (102 to 1,219 mm).", "piece-w"); err != nil {
			return nil, err
		}
		if err := num.Range(c.PieceL, pieceLMin, pieceLMax, "Enter a piece length from 12 to 120 in (305 to 3,048 mm).", "piece-l"); err != nil {
			return nil, err
		}
		pieceW, pieceL = c.PieceW, c.PieceL
	}
	pieceMM2 := pieceW * pieceL
	pieceSqFt := pieceMM2 / SqFt
	var perPallet float64
	if c.PalletMode == PalletBySqFt {
		if err := num.Range(c.PalletSqFt, 50.0, 2000.0, "Enter a pallet coverage from 50 to 2,000 sq ft.", "pallet-sqft"); err != nil {
			return nil, err
		}
		perPallet = num.FloorRel(c.PalletSqFt / pieceSqFt)
		if perPallet < 1.0 {
			return nil, num.NewError("The pallet holds less than one piece of this size.", "pallet-sqft")
		}
	} else {
		if c.PalletPieces != math.Trunc(c.PalletPieces) || c.PalletPieces < 1.0 || c.PalletPieces > 2000.0 {
			return nil, num.NewError("Enter a whole number of pieces per pallet from 1 to 2,000.", "pallet-pieces")
		}
		perPallet = c.PalletPieces
	}
	if err := num.Range(c.Allowance, 0.0, 30.0, "Enter an allowance from 0 to 30 %.", "allowance"); err != nil {
		return nil, err
	}
	if err := num.Range(c.Spare, 0.0, 30.0, "Enter a spare from 0 to 30 %.", "spare"); err != nil {
		return nil, err
	}
	plugs := c.Method == MethodPlugs
	warnings := []Warning{}
	areas := make([]AreaPlan, 0, len(c.Areas))
	for _, a := range c.Areas {
		f := func(k string) string { return a.ID + "-" + k }
		length := func(v float64, k, label string, max float64) error {
			msg := fmt.Sprintf("Enter %s from 1 to %s ft (0.3 to %s m).", label, shortest(max/calculiva.Foot), num.Locale(max/1000.0, 1))
			return num.Range(v, calculiva.Foot, max, msg, f(k))
		}
		copies := func(count float64) error {
			if count != math.Trunc(count) || !(count >= 1.0 && count <= 20.0) {
				return num.NewError("Enter a whole number of copies from 1 to 20.", f("count"))
			}
			return nil
		}
		count := 1.0
		var layout *RectLayout
		noLayout := false
		var areaMM2 float64
		switch s := a.Shape.(type) {
		case Rect:
			if err := copies(s.Count); err != nil {
				return nil, err
			}
			count = s.Count
			if err := length(s.L, "l", "a length", rectMax); err != nil {
				return nil, err
			}
			if err := length(s.W, "w", "a width", rectMax); err != nil {
				return nil, err
			}
			// Beyond 2,500 placed pieces, the drawing gives way to a count by area.
			if !plugs {
				if num.CeilRel(s.L/pieceL+1.0)*num.CeilRel(s.W/pieceW) > maxPieces {
					noLayout = true
					warnings = append(warnings, Warning{Kind: NoLayout, Area: a.ID})
				} else {
					lay, err := LayRect(s.L, s.W, pieceW, pieceL)
					if err != nil {
						return nil, err
					}
					layout = lay
				}
			}
			areaMM2 = s.L * s.W
		case Circle:
			if err := copies(s.Count); err != nil {
				return nil, err
			}
			count = s.Count
			if err := length(s.R, "r", "a radius", 150.0*calculiva.Foot); err != nil {
				return nil, err
			}
			areaMM2 = math.Pi * s.R * s.R
		case Triangle:
			if err := copies(s.Count); err != nil {
				return nil, err
			}
			count = s.Count
			if err := length(s.B, "b", "a base", rectMax); err != nil {
				return nil, err
			}
			if err := length(s.H, "h", "a height", rectMax); err != nil {
				return nil, err
			}
			areaMM2 = s.B * s.H / 2.0
		case KnownArea:
			if err := num.Range(s.Area, 0.1*sqM, 10000.0*sqM, "Enter an area from 0.1 to 10,000 m² (about 1.1 to 107,639 sq ft).", f("area")); err != nil {
				return nil, err
			}
			areaMM2 = s.Area
		default:
			return nil, num.NewError("Choose a shape.", f("shape"))
		}
		sqft := areaMM2 * count / SqFt
		plan := AreaPlan{ID: a.ID, Shape: a.Shape, Count: count, AreaMM2: areaMM2, SqFt: sqft, Mode: ModePlugs}
		switch {
		case plugs:
		case layout != nil:
			b := float64(layout.Bought)
			plan.Mode = ModeLayout
			plan.Bought = b * count
			plan.ExtraPct = ptr(tidy((b*pieceMM2 - areaMM2) / areaMM2 * 100.0))
			plan.Layout = layout
		default:
			bought := num.CeilRel(sqft * (1.0 + c.Allowance/100.0) / pieceSqFt)
			plan.Mode = ModeArea
			if noLayout {
				plan.Mode = ModeNoLayout
			}
			plan.Bought = bought
			plan.ExtraPct = ptr(tidy((bought*pieceSqFt - sqft) / sqft * 100.0))
		}
		areas = append(areas, plan)
	}
	areaMM2, layoutBought, areaBought := 0.0, 0.0, 0.0
	for _, a := range areas {
		areaMM2 += float64(a.AreaMM2 * a.Count)
		switch a.Mode {
		case ModeLayout:
			layoutBought += a.Bought
		case ModeArea, ModeNoLayout:
			areaBought += a.Bought
		}
	}
	sqft := areaMM2 / SqFt
	spare := 0.0
	if !plugs {
		spare = num.CeilRel(layoutBought * c.Spare / 100.0)
	}
	flat := num.CeilRel(sqft * (1.0 + rules.Allowance/100.0) / pieceSqFt)
	var plug *PlugPlan
	if plugs {
		var sp *Species
		if c.Species != "custom" {
			for i := range rules.Species {
				if rules.Species[i].ID == c.Species {
					found := rules.Species[i]
					sp = &found
					break
				}
			}
			if sp == nil {
				return nil, num.NewError("Choose a grass species.", "species")
			}
		}
		if err := num.Range(c.Spacing, 4.0*calculiva.Inch, 24.0*calculiva.Inch, "Enter a plug spacing from 4 to 24 in (102 to 610 mm).", "spacing"); err != nil {
			return nil, err
		}
		offered := false
		for _, size := range rules.PlugSizes {
			if size == c.PlugSize {
				offered = true
				break
			}
		}
		if !offered {
			return nil, num.NewError("Choose a 2, 3 or 4 in plug.", "plug-size")
		}
		plugMM := c.PlugSize * calculiva.Inch
		count := num.CeilRel(areaMM2 / (c.Spacing * c.Spacing))
		sodSqFt := count * plugMM * plugMM / SqFt
		var tableSqFt, tablePieces *[2]float64
		if sp != nil && c.PlugSize == sp.PlugIn {
			tableSqFt = &[2]float64{sp.SodSqFtPer1000[0] * sqft / 1000.0, sp.SodSqFtPer1000[1] * sqft / 1000.0}
			tablePieces = &[2]float64{num.CeilRel(tableSqFt[0] / pieceSqFt), num.CeilRel(tableSqFt[1] / pieceSqFt)}
		}
		if sp != nil && math.Abs(c.Spacing-sp.SpacingMM) > eps {
			warnings = append(warnings, Warning{Kind: Spacing, Species: sp.Species, Spacing: c.Spacing, TableMM: sp.SpacingMM})
		}
		plug = &PlugPlan{
			Species:     sp,
			Count:       count,
			SodSqFt:     sodSqFt,
			SodSqYd:     sodSqFt * SqFt / SqYd,
			PerSqYd:     SqYd / (plugMM * plugMM),
			Pieces:      num.CeilRel(sodSqFt / pieceSqFt),
			TableSqFt:   tableSqFt,
			TablePieces: tablePieces,
		}
	}
	pieces := layoutBought + areaBought + spare
	if plug != nil {
		pieces = plug.Pieces
	}
	full := num.FloorRel(pieces / perPallet)
	loose := pieces - float64(full*perPallet)
	palletsUp := num.CeilRel(pieces / perPallet)
	leftover := float64(palletsUp*perPallet) - pieces
	// Soil preparation over the whole lawn: topsoil (Penn State), organic matter as a layer
	// (Penn State) or as a rate per 1,000 sq ft (NC State), with the other publisher's equivalent.
	if err := num.Range(c.Topsoil, 0.0, 12.0*calculiva.Inch, "Enter a topsoil depth from 0 to 12 in (0 to 305 mm).", "topsoil"); err != nil {
		return nil, err
	}
	organicMM3 := 0.0
	var equivalentRate, equivalentLayerMM *float64
	switch c.Organic {
	case OrganicLayer:
		if err := num.Range(c.OrganicIn, 0.0, 4.0*calculiva.Inch, "Enter an organic layer from 0 to 4 in (0 to 102 mm).", "organic-in"); err != nil {
			return nil, err
		}
		organicMM3 = areaMM2 * c.OrganicIn
		equivalentRate = ptr(organicMM3 / CuYd / (sqft / 1000.0))
		if c.OrganicIn > rules.OrganicLayerMaxMM+eps {
			warnings = append(warnings, Warning{Kind: OrganicThick, Layer: c.OrganicIn, MaxMM: rules.OrganicLayerMaxMM})
		}
	case OrganicRate:
		if err := num.Range(c.OrganicRate, 0.0, 4.0, "Enter a rate from 0 to 4 cu yd per 1,000 sq ft.", "organic-rate"); err != nil {
			return nil, err
		}
		organicMM3 = c.OrganicRate * sqft / 1000.0 * CuYd
		equivalentLayerMM = ptr(organicMM3 / areaMM2)
		lo, hi := rules.OrganicRate[0], rules.OrganicRate[1]
		if c.OrganicRate < lo-eps || c.OrganicRate > hi+eps {
			warnings = append(warnings, Warning{Kind: RateOutside, Rate: c.OrganicRate, Range: rules.OrganicRate})
		}
	}
	if c.Topsoil > eps && c.Topsoil < rules.TopsoilMM[0]-eps {
		warnings = append(warnings, Warning{Kind: TopsoilThin, Topsoil: c.Topsoil, MinMM: rules.TopsoilMM[0]})
	}
	topsoilMM3 := areaMM2 * c.Topsoil
	prep := Prep{
		TopsoilMM3:        topsoilMM3,
		TopsoilCuFt:       topsoilMM3 / CuFt,
		TopsoilCuYd:       topsoilMM3 / CuYd,
		OrganicMM3:        organicMM3,
		OrganicCuFt:       organicMM3 / CuFt,
		OrganicCuYd:       organicMM3 / CuYd,
		EquivalentRate:    equivalentRate,
		EquivalentLayerMM: equivalentLayerMM,
	}
	// Cost: whole pallets, or full pallets plus loose pieces when priced per piece or per sq ft.
	if err := num.Range(c.Price, 0.0, 10000.0, "Enter a price from 0 to 10,000 USD.", "price"); err != nil {
		return nil, err
	}
	if err := num.Range(c.Delivery, 0.0, 10000.0, "Enter a delivery charge from 0 to 10,000 USD.", "delivery"); err != nil {
		return nil, err
	}
	var perPiece, whole, mixed *float64
	switch c.PriceMode {
	case PricePerPiece:
		perPiece = ptr(c.Price)
	case PricePerSqFt:
		perPiece = ptr(c.Price * pieceSqFt)
	}
	if c.Price != 0.0 {
		if perPiece == nil {
			whole = ptr(float64(palletsUp*c.Price) + c.Delivery)
		} else {
			whole = ptr(float64(palletsUp*perPallet**perPiece) + c.Delivery)
			mixed = ptr(float64(pieces**perPiece) + c.Delivery)
		}
	}
	// Weight: a range per pallet, never a single number.
	if c.Payload != 0.0 {
		if err := num.Range(c.Payload, 100.0, 10000.0, "Enter a payload from 100 to 10,000 lb, or 0.", "payload"); err != nil {
			return nil, err
		}
	}
	weightLb := [2]float64{rules.PalletLb[0] * palletsUp, rules.PalletLb[1] * palletsUp}
	var trips *[2]float64
	if c.Payload != 0.0 {
		trips = &[2]float64{num.CeilRel(weightLb[0] / c.Payload), num.CeilRel(weightLb[1] / c.Payload)}
	}
	return &Plan{
		PieceW:    pieceW,
		PieceL:    pieceL,
		PieceMM2:  pieceMM2,
		PieceSqFt: pieceSqFt,
		PerPallet: perPallet,
		Areas:     areas,
		Totals:    Totals{AreaMM2: areaMM2, SqFt: sqft, LayoutBought: layoutBought, AreaBought: areaBought, Spare: spare, Pieces: pieces, Flat: flat},
		Pallets:   Pallets{PerPallet: perPallet, Full: full, Loose: loose, PalletsUp: palletsUp, Leftover: leftover},
		Plug:      plug,
		Prep:      prep,
		Cost:      Cost{PerPiece: perPiece, Whole: whole, Mixed: mixed},
		WeightLb:  weightLb,
		Trips:     trips,
		Warnings:  warnings,
	}, nil
}

// FormatTableRow is one row of [FormatTable].
type FormatTableRow struct {
	// Format is the format.
	Format Format
	// SqFt is the exact square feet per piece.
	SqFt float64
	// PerPallet is the number of pieces for each pallet coverage asked.
	PerPallet []float64
	// ForLawn is the number of pieces for the lawn.
	ForLawn float64
}

// FormatTable returns the formats: square feet per piece, pieces for each
// pallet coverage in soldAs, pieces for the lawn.
func FormatTable(rules Rules, soldAs []float64, sqft float64) []FormatTableRow {
	out := make([]FormatTableRow, 0, len(rules.Formats))
	for _, f := range rules.Formats {
		s := f.WidthMM * f.LengthMM / SqFt
		perPallet := make([]float64, 0, len(soldAs))
		for _, x := range soldAs {
			perPallet = append(perPallet, num.CeilRel(x/s))
		}
		out = append(out, FormatTableRow{Format: f, SqFt: s, PerPallet: perPallet, ForLawn: num.CeilRel(sqft / s)})
	}
	return out
}
