// Package asphalt estimates hot mix asphalt by zone and by course.
//
// Lengths are millimetres; weights are pounds and short tons because that is
// how a U.S. plant quotes. No unit weight, lift rule or reference value lives in
// the code: the in-place unit weight is a required input, and the water unit
// weight, the lift rules and the references are fields of [Rules] that the user
// sets. The results are planning quantities: the mix design, the plant and the
// paving crew govern.
//
// Calculator page: https://calculiva.com/landscaping/asphalt-calculator/
package asphalt

import (
	"fmt"
	"math"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/internal/num"
)

const eps = 1e-7

// SqFt is one square foot in mm².
const SqFt = calculiva.Foot * calculiva.Foot

// CuFt is one cubic foot in mm³.
const CuFt = calculiva.Foot * calculiva.Foot * calculiva.Foot

// SqYd is one square yard in mm².
const SqYd = SqFt * 9.0

// CuYd is one cubic yard in mm³.
const CuYd = CuFt * 27.0

// Bounds of a zone length, mm.
const (
	minMM = 150.0
	maxMM = 200000.0
)

// PoundsPerShortTon is the number of pounds in a short ton.
//
// Data sheet soil-weights-and-sale-units (physical_basis.pounds_per_short_ton),
// source https://www.nist.gov/document/2026-hb-130-section-iv-b, verified 2026-09-25.
const PoundsPerShortTon float64 = 2000.0

// Reference is an in-place unit weight entered by the user for comparison.
type Reference struct {
	// ID is the reference id.
	ID string
	// Label describes the reference.
	Label string
	// PCF is the unit weight, lb per cubic foot.
	PCF float64
	// Publisher names who published it.
	Publisher string
	// URL is the page read.
	URL string
	// Stated tells whether the value is stated on the page (otherwise it was
	// read in the tool's script).
	Stated bool
}

// Rules are the rules of the asphalt calculator. Apart from PoundsPerTon, every
// field is an entry of the user, read on the mix design, the paving
// specification or the source of their choice: no default is supplied. The
// sources Calculiva read are listed on https://calculiva.com/data-sources/.
type Rules struct {
	// WaterPCF is the unit weight of water, lb per cu ft, used by [MixDesign].
	WaterPCF float64
	// PoundsPerTon is the number of pounds per short ton.
	PoundsPerTon float64
	// MinLiftMultiple is the minimum lift as a multiple of the aggregate size,
	// used when a course has an aggregate size.
	MinLiftMultiple float64
	// MaxLiftStaticIn is the maximum lift under a static roller, inches.
	MaxLiftStaticIn float64
	// MaxLiftVibratoryIn is the maximum lift under a vibratory roller, inches.
	MaxLiftVibratoryIn float64
	// References are the reference unit weights to compare with, possibly none.
	References []Reference
}

// NewRules returns the asphalt rules with [PoundsPerShortTon] set. The water
// unit weight, the lift rules and the references are left for the user to set.
func NewRules() Rules {
	return Rules{PoundsPerTon: PoundsPerShortTon}
}

func positive(v float64, msg, field string) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return num.NewError(msg, field)
	}
	return nil
}

func finite(v, min, max float64, label, field string) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < min-eps || v > max+eps {
		return num.NewError(fmt.Sprintf("Enter %s from %s to %s.", label, num.JSNum(min), num.JSNum(max)), field)
	}
	return nil
}

// Shape is the shape of a paved zone, lengths in mm: [Rect], [Flare] or [Round].
type Shape interface {
	isShape()
}

// Rect is a rectangle.
type Rect struct {
	// W is the width.
	W float64
	// L is the length.
	L float64
}

// Flare is a trapezoid: two parallel widths over a depth.
type Flare struct {
	// W is the narrow width.
	W float64
	// W2 is the wide width.
	W2 float64
	// L is the depth of the flare.
	L float64
}

// Round is a full, half or quarter disc (Part = 1, 0.5 or 0.25).
type Round struct {
	// R is the radius.
	R float64
	// Part is the fraction of the disc.
	Part float64
}

func (Rect) isShape()  {}
func (Flare) isShape() {}
func (Round) isShape() {}

// Zone is a paved zone.
type Zone struct {
	// ID is the zone id; it prefixes error fields.
	ID string
	// Shape is the zone's shape.
	Shape Shape
}

// ZoneArea returns the area of one zone, mm².
func ZoneArea(z Zone) (float64, error) {
	f := func(k string) string { return z.ID + "-" + k }
	switch s := z.Shape.(type) {
	case Rect:
		if err := finite(s.W, minMM, maxMM, "a width", f("w")); err != nil {
			return 0, err
		}
		if err := finite(s.L, minMM, maxMM, "a length", f("l")); err != nil {
			return 0, err
		}
		return s.W * s.L, nil
	case Flare:
		if err := finite(s.W, minMM, maxMM, "the narrow width", f("w")); err != nil {
			return 0, err
		}
		if err := finite(s.W2, minMM, maxMM, "the wide width", f("w2")); err != nil {
			return 0, err
		}
		if err := finite(s.L, minMM, maxMM, "the depth of the flare", f("l")); err != nil {
			return 0, err
		}
		return (s.W + s.W2) / 2.0 * s.L, nil
	case Round:
		if err := finite(s.R, minMM, maxMM, "a radius", f("r")); err != nil {
			return 0, err
		}
		if s.Part != 1.0 && s.Part != 0.5 && s.Part != 0.25 {
			return 0, num.NewError("Choose a full, half or quarter circle.", f("part"))
		}
		return math.Pi * s.R * s.R * s.Part, nil
	default:
		return 0, num.NewError("Choose a zone shape.", f("shape"))
	}
}

// UnitWeight is the in-place unit weight: entered directly ([PCF]), or from the
// mix design ([MixDesign]).
type UnitWeight interface {
	isUnitWeight()
}

// PCF is a unit weight in lb per cubic foot (90 to 170).
type PCF float64

// MixDesign is a Rice value (Gmm, 2 to 3) and a compaction in percent (85 to 100).
type MixDesign struct {
	// Rice is the Rice value.
	Rice float64
	// Compaction is the compaction, percent.
	Compaction float64
}

func (PCF) isUnitWeight()       {}
func (MixDesign) isUnitWeight() {}

// RicePCF returns the in-place unit weight in lb per cubic foot from the mix
// design: Rice value (maximum theoretical specific gravity, Gmm) x compaction
// (percent of the maximum density) x unit weight of water (lb per cubic foot).
// All three are entries of the user.
func RicePCF(rice, compactionPercent, waterPCF float64) float64 {
	return rice * waterPCF * compactionPercent / 100.0
}

// UnitWeightOf returns the in-place unit weight in lb per cubic foot: entered,
// or from the mix design with [RicePCF].
func UnitWeightOf(u UnitWeight, waterPCF float64) (float64, error) {
	switch w := u.(type) {
	case MixDesign:
		if err := finite(w.Rice, 2.0, 3.0, "a Rice value (Gmm)", "rice"); err != nil {
			return 0, err
		}
		if err := finite(w.Compaction, 85.0, 100.0, "a compaction in percent", "compaction"); err != nil {
			return 0, err
		}
		if err := positive(waterPCF, "Enter the unit weight of water in lb per cubic foot.", "water"); err != nil {
			return 0, err
		}
		return RicePCF(w.Rice, w.Compaction, waterPCF), nil
	case PCF:
		if err := finite(float64(w), 90.0, 170.0, "a unit weight in lb per cubic foot", "pcf"); err != nil {
			return 0, err
		}
		return float64(w), nil
	default:
		return 0, num.NewError("Enter a unit weight or a mix design.", "unit")
	}
}

// Course is a course (layer) of asphalt.
type Course struct {
	// ID is the course id; it prefixes error fields.
	ID string
	// Thickness is the compacted thickness, mm.
	Thickness float64
	// NMAS is the nominal maximum aggregate size, mm, 0 if unknown.
	NMAS float64
}

// Roller is the roller type.
type Roller int

const (
	// Static is a static steel-wheeled roller.
	Static Roller = iota
	// Vibratory is a pneumatic or vibratory roller.
	Vibratory
)

// Input is the input of [NewPlan].
type Input struct {
	// Zones holds one to eight zones.
	Zones []Zone
	// Courses holds one to three courses.
	Courses []Course
	// Unit is the unit weight.
	Unit UnitWeight
	// Roller is the roller type.
	Roller Roller
	// Extra is the allowance, percent (0 to 25).
	Extra float64
	// Load is the truck load, tons, 0 for none.
	Load float64
	// Price is the price per ton.
	Price float64
}

// NewInput returns an input with the calculator defaults: static roller, no
// allowance, no load, no price.
func NewInput(zones []Zone, courses []Course, unit UnitWeight) Input {
	return Input{Zones: zones, Courses: courses, Unit: unit, Roller: Static}
}

// ZonePlan is a zone's area.
type ZonePlan struct {
	// ID is the zone id.
	ID string
	// Area is the area, mm².
	Area float64
	// SqFt is the area, sq ft.
	SqFt float64
	// SqYd is the area, sq yd.
	SqYd float64
}

// ZoneTons is the tons of one course on one zone.
type ZoneTons struct {
	// ID is the zone id.
	ID string
	// Tons is the weight, short tons.
	Tons float64
}

// CoursePlan is one course's result.
type CoursePlan struct {
	// ID is the course id.
	ID string
	// Thickness is the thickness, mm.
	Thickness float64
	// NMAS is the aggregate size, mm.
	NMAS float64
	// Volume is the volume, mm³.
	Volume float64
	// CuFt is the volume, cu ft.
	CuFt float64
	// CuYd is the volume, cu yd.
	CuYd float64
	// Tons is the weight, short tons.
	Tons float64
	// Lifts is the number of lifts needed under the roller.
	Lifts float64
	// Lift is the thickness of each lift, mm.
	Lift float64
	// MinLift is the minimum lift, mm (0 without aggregate size).
	MinLift float64
	// TooThin tells that the lift is thinner than the minimum.
	TooThin bool
	// Split tells that the course takes more than one lift.
	Split bool
	// ByZone is the tons by zone.
	ByZone []ZoneTons
	// SqFtPerTon is the square feet per ton at this thickness.
	SqFtPerTon float64
}

// ReferenceTons is a reference unit weight applied to the job.
type ReferenceTons struct {
	// Reference is the reference.
	Reference Reference
	// Tons is the tons at that unit weight.
	Tons float64
	// Delta is the difference with the job's tons.
	Delta float64
}

// WarningKind is the short kind of a warning, as used on the calculator page.
type WarningKind string

const (
	// Thin flags a lift thinner than the minimum lift of the rules.
	Thin WarningKind = "thin"
	// Split flags a course laid in more than one lift.
	Split WarningKind = "split"
)

// Warning is a course warning.
type Warning struct {
	// Kind is the kind of warning.
	Kind WarningKind
	// Course is the course id.
	Course string
}

// Plan is the result of [NewPlan].
type Plan struct {
	// PCF is the unit weight used, lb per cu ft.
	PCF float64
	// LbPerSqYdIn is the same, lb per sq yd per inch.
	LbPerSqYdIn float64
	// TonsPerCuYd is the tons per cubic yard.
	TonsPerCuYd float64
	// Zones are the zones.
	Zones []ZonePlan
	// Area is the paved area, mm².
	Area float64
	// SqFt is the paved area, sq ft.
	SqFt float64
	// SqYd is the paved area, sq yd.
	SqYd float64
	// Courses are the courses.
	Courses []CoursePlan
	// Volume is the volume, mm³.
	Volume float64
	// CuFt is the volume, cu ft.
	CuFt float64
	// CuYd is the volume, cu yd.
	CuYd float64
	// Tons is the weight, short tons.
	Tons float64
	// Ordered is the tons with the allowance.
	Ordered float64
	// Loads is the number of whole truck loads (0 without a load size).
	Loads float64
	// Delivered is the tons delivered.
	Delivered float64
	// Surplus is delivered minus ordered.
	Surplus float64
	// Cost is delivered x price.
	Cost float64
	// References are the reference unit weights applied to the job.
	References []ReferenceTons
	// Spread is the spread between the heaviest and lightest reference, tons.
	Spread float64
	// Warnings are the course warnings.
	Warnings []Warning
}

// NewPlan plans asphalt: tons by zone and course, lifts, order in whole loads,
// cost and the comparison with the references of the rules.
func NewPlan(c Input, rules Rules) (*Plan, error) {
	if len(c.Zones) == 0 || len(c.Zones) > 8 {
		return nil, num.NewError("Enter one to eight zones.", "zones")
	}
	if len(c.Courses) == 0 || len(c.Courses) > 3 {
		return nil, num.NewError("Enter one to three courses.", "courses")
	}
	if err := finite(c.Extra, 0.0, 25.0, "an allowance in percent", "extra"); err != nil {
		return nil, err
	}
	if err := finite(c.Price, 0.0, 100000.0, "a quote per ton", "price"); err != nil {
		return nil, err
	}
	if c.Load != 0.0 {
		if err := finite(c.Load, 1.0, 40.0, "a load size in tons, or 0", "load"); err != nil {
			return nil, err
		}
	}
	pcf, err := UnitWeightOf(c.Unit, rules.WaterPCF)
	if err != nil {
		return nil, err
	}
	lbPerTon := rules.PoundsPerTon
	maxLift := rules.MaxLiftStaticIn
	if c.Roller == Vibratory {
		maxLift = rules.MaxLiftVibratoryIn
	}
	if err := positive(maxLift, "Enter the maximum lift for the roller, in inches.", "max-lift"); err != nil {
		return nil, err
	}
	maxLift *= calculiva.Inch
	zones := make([]ZonePlan, 0, len(c.Zones))
	for _, z := range c.Zones {
		area, err := ZoneArea(z)
		if err != nil {
			return nil, err
		}
		zones = append(zones, ZonePlan{ID: z.ID, Area: area, SqFt: area / SqFt, SqYd: area / SqYd})
	}
	area := 0.0
	for _, z := range zones {
		area += z.Area
	}
	tonsOf := func(volume, w float64) float64 { return volume / CuFt * w / lbPerTon }
	courses := make([]CoursePlan, 0, len(c.Courses))
	for _, k := range c.Courses {
		f := func(n string) string { return k.ID + "-" + n }
		if err := finite(k.Thickness, 0.5*calculiva.Inch, 12.0*calculiva.Inch, "a compacted thickness", f("thickness")); err != nil {
			return nil, err
		}
		if k.NMAS != 0.0 {
			if err := finite(k.NMAS, 4.0, 40.0, "an aggregate size in mm, or 0", f("nmas")); err != nil {
				return nil, err
			}
			if err := positive(rules.MinLiftMultiple, "Enter the minimum lift as a multiple of the aggregate size.", "min-lift"); err != nil {
				return nil, err
			}
		}
		volume := area * k.Thickness
		lifts := num.CeilULP(k.Thickness/maxLift - eps)
		lift := k.Thickness / lifts
		minLift := 0.0
		if k.NMAS != 0.0 {
			minLift = k.NMAS * rules.MinLiftMultiple
		}
		byZone := make([]ZoneTons, 0, len(zones))
		for _, z := range zones {
			byZone = append(byZone, ZoneTons{ID: z.ID, Tons: tonsOf(z.Area*k.Thickness, pcf)})
		}
		courses = append(courses, CoursePlan{
			ID:         k.ID,
			Thickness:  k.Thickness,
			NMAS:       k.NMAS,
			Volume:     volume,
			CuFt:       volume / CuFt,
			CuYd:       volume / CuYd,
			Tons:       tonsOf(volume, pcf),
			Lifts:      lifts,
			Lift:       lift,
			MinLift:    minLift,
			TooThin:    k.NMAS != 0.0 && lift < minLift-eps,
			Split:      lifts > 1.0,
			ByZone:     byZone,
			SqFtPerTon: lbPerTon / (pcf * k.Thickness / calculiva.Foot),
		})
	}
	volume, tons := 0.0, 0.0
	for _, k := range courses {
		volume += k.Volume
		tons += k.Tons
	}
	ordered := tons * (1.0 + c.Extra/100.0)
	loads, delivered := 0.0, ordered
	if c.Load != 0.0 {
		loads = num.CeilULP(ordered / c.Load)
		delivered = loads * c.Load
	}
	references := make([]ReferenceTons, 0, len(rules.References))
	for _, r := range rules.References {
		references = append(references, ReferenceTons{Reference: r, Tons: tonsOf(volume, r.PCF), Delta: tonsOf(volume, r.PCF) - tons})
	}
	spread := 0.0
	if len(references) > 0 {
		hi, lo := math.Inf(-1), math.Inf(1)
		for _, r := range references {
			if r.Tons > hi {
				hi = r.Tons
			}
			if r.Tons < lo {
				lo = r.Tons
			}
		}
		spread = hi - lo
	}
	warnings := []Warning{}
	for _, k := range courses {
		if k.TooThin {
			warnings = append(warnings, Warning{Kind: Thin, Course: k.ID})
		}
		if k.Split {
			warnings = append(warnings, Warning{Kind: Split, Course: k.ID})
		}
	}
	return &Plan{
		PCF:         pcf,
		LbPerSqYdIn: pcf * 9.0 / 12.0,
		TonsPerCuYd: pcf * 27.0 / lbPerTon,
		Zones:       zones,
		Area:        area,
		SqFt:        area / SqFt,
		SqYd:        area / SqYd,
		Courses:     courses,
		Volume:      volume,
		CuFt:        volume / CuFt,
		CuYd:        volume / CuYd,
		Tons:        tons,
		Ordered:     ordered,
		Loads:       loads,
		Delivered:   delivered,
		Surplus:     delivered - ordered,
		Cost:        delivered * c.Price,
		References:  references,
		Spread:      spread,
		Warnings:    warnings,
	}, nil
}

// TonCoverage is the coverage of one ton at one thickness.
type TonCoverage struct {
	// Inches is the thickness, inches.
	Inches float64
	// SqFtPerTon is the square feet per ton.
	SqFtPerTon float64
	// SqYdPerTon is the square yards per ton.
	SqYdPerTon float64
}

// CoverageTable returns the square feet covered by one ton at each thickness,
// for the unit weight in use.
func CoverageTable(pcf float64, thicknessesIn []float64, poundsPerTon float64) []TonCoverage {
	out := make([]TonCoverage, 0, len(thicknessesIn))
	for _, t := range thicknessesIn {
		out = append(out, TonCoverage{
			Inches:     t,
			SqFtPerTon: poundsPerTon / (pcf * t / 12.0),
			SqYdPerTon: poundsPerTon / (pcf * t / 12.0) / 9.0,
		})
	}
	return out
}
