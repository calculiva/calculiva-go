// Package mulch estimates mulch by area: beds, tree rings, known areas, play
// structures and swings (CPSC).
//
// Lengths are millimetres; sale volumes are cubic feet and cubic yards. Mulch
// is sold by volume: no weight here. Every published depth, trunk gap and CPSC
// rule comes from [Rules], built by [NewRules] from the constants below. The
// results are planning quantities: the product label, the publishers quoted and
// the site govern.
//
// Calculator page: https://calculiva.com/landscaping/mulch-calculator/
package mulch

import (
	"fmt"
	"math"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/internal/num"
	"github.com/calculiva/calculiva-go/soil"
)

const eps = 1e-7

// Bounds of a mulch depth, mm, and the messages that state them.
const (
	depthMin    = 3.0
	depthMax    = 600.0
	depthMsg    = "Enter a depth from 1/8 in to about 24 in (3 to 600 mm)."
	existingMsg = "Enter the depth already in place from 0 to about 24 in (600 mm)."
)

// TreeRadius is a minimum ring radius for one tree size.
type TreeRadius struct {
	// Size is the tree size: small, medium or large.
	Size string
	// Ft is the minimum ring radius, feet.
	Ft float64
}

// Publisher is a publisher's mulch guidance: depth range, trunk gap and ring
// radii where published. A nil field means the publisher gives no such figure.
type Publisher struct {
	// ID is the publisher id.
	ID string
	// Name is the publisher name.
	Name string
	// SourceURL is the page read.
	SourceURL string
	// DepthIn is the depth range for coarse mulch, inches.
	DepthIn *[2]float64
	// FineDepthIn is the depth range for fine mulch, inches.
	FineDepthIn *[2]float64
	// TrunkGapIn is the bare gap at the trunk, inches (a range counts by its
	// minimum).
	TrunkGapIn *[2]float64
	// RingRadiusByTreeFt is the minimum ring radius by tree size (small,
	// medium, large), feet.
	RingRadiusByTreeFt []TreeRadius
}

// Publishers is the mulch depth and trunk-gap guidance, publisher by publisher.
// A single published value v is stored as the range [v, v]; Casey Trees and
// Maple Grove publish it inside their 3-3-3 rule.
//
// Data sheet mulch-depth-guidance (publishers), source https://extension.psu.edu/mulching-landscape-trees/,
// verified 2026-09-25.
var Publishers = []Publisher{
	{ID: "psu", Name: "Penn State Extension", SourceURL: "https://extension.psu.edu/mulching-landscape-trees/", DepthIn: &[2]float64{2.0, 4.0}, FineDepthIn: &[2]float64{1.0, 2.0}},
	{ID: "umd", Name: "University of Maryland Extension", SourceURL: "https://extension.umd.edu/resource/mulching-trees-and-shrubs", DepthIn: &[2]float64{1.0, 3.0}, TrunkGapIn: &[2]float64{3.0, 3.0}},
	{ID: "isu", Name: "Iowa State University Extension", SourceURL: "https://yardandgarden.extension.iastate.edu/how-to/using-mulch-garden", DepthIn: &[2]float64{2.0, 4.0}},
	{ID: "ucm", Name: "UC Marin Master Gardeners (UC ANR)", SourceURL: "https://ucanr.edu/site/uc-marin-master-gardeners/applying-mulch", TrunkGapIn: &[2]float64{6.0, 12.0}},
	{ID: "ncufc", Name: "NCUFC", SourceURL: "https://www.ncufc.org/proper-mulching-for-trees.php", DepthIn: &[2]float64{2.0, 4.0}, TrunkGapIn: &[2]float64{1.0, 2.0}, RingRadiusByTreeFt: []TreeRadius{{Size: "small", Ft: 3.0}, {Size: "medium", Ft: 8.0}, {Size: "large", Ft: 12.0}}},
	{ID: "casey", Name: "Casey Trees", SourceURL: "https://caseytrees.org/2024/05/mulching-dos-and-donts/", TrunkGapIn: &[2]float64{3.0, 3.0}},
	{ID: "maplegrove", Name: "City of Maple Grove, Minnesota", SourceURL: "https://www.maplegrovemn.gov/DocumentCenter/View/1996/How-to-apply-mulch-around-trees-PDF", TrunkGapIn: &[2]float64{3.0, 3.0}},
}

// CPSCAllDirectionsFt is how far surfacing must extend from play equipment in
// all directions, feet (CPSC 324).
//
// Data sheet playground-surfacing-cpsc (home_handbook.extent.all_directions_ft), source
// https://www.cpsc.gov/s3fs-public/324.pdf, verified 2026-09-25.
const CPSCAllDirectionsFt float64 = 6.0

// CPSCSwingMultiple is, for to-fro swings, the surfacing front and back as a
// multiple of the top bar height (CPSC 324).
//
// Data sheet playground-surfacing-cpsc (home_handbook.extent.swing_front_back_multiple_of_top_bar),
// source https://www.cpsc.gov/s3fs-public/324.pdf, verified 2026-09-25.
const CPSCSwingMultiple float64 = 2.0

// CPSCTireSwingExtraFt is, for tire swings, the feet added to the chain height
// (CPSC 324).
//
// Data sheet playground-surfacing-cpsc (home_handbook.extent.tire_swing_extra_ft), source
// https://www.cpsc.gov/s3fs-public/324.pdf, verified 2026-09-25.
const CPSCTireSwingExtraFt float64 = 6.0

// CPSCWoodMulchMaxEquipmentFt is the tallest equipment covered by wood mulch or
// chips in the home handbook, feet (CPSC 324).
//
// Data sheet playground-surfacing-cpsc (home_handbook.loose_fill.wood_mulch_chips_max_equipment_ft),
// source https://www.cpsc.gov/s3fs-public/324.pdf, verified 2026-09-25.
const CPSCWoodMulchMaxEquipmentFt float64 = 8.0

// CPSCLowEquipmentFt is the height below which equipment is "low", feet
// (CPSC 324).
//
// Data sheet playground-surfacing-cpsc (home_handbook.loose_fill.low_equipment_ft), source
// https://www.cpsc.gov/s3fs-public/324.pdf, verified 2026-09-25.
const CPSCLowEquipmentFt float64 = 4.0

// CPSCMaintainedDepthIn is the maintained loose-fill depth, inches (CPSC 324).
//
// Data sheet playground-surfacing-cpsc (home_handbook.loose_fill.maintained_depth_in), source
// https://www.cpsc.gov/s3fs-public/324.pdf, verified 2026-09-25.
const CPSCMaintainedDepthIn float64 = 9.0

// CPSCInitialFillIn is the initial fill that compresses to the maintained
// depth, inches (CPSC 324).
//
// Data sheet playground-surfacing-cpsc (home_handbook.loose_fill.initial_fill_in), source
// https://www.cpsc.gov/s3fs-public/324.pdf, verified 2026-09-25.
const CPSCInitialFillIn float64 = 12.0

// CPSCLowEquipmentMinDepthIn is the minimum surfacing under low equipment,
// inches (CPSC 324).
//
// Data sheet playground-surfacing-cpsc (home_handbook.loose_fill.low_equipment_min_depth_in),
// source https://www.cpsc.gov/s3fs-public/324.pdf, verified 2026-09-25.
const CPSCLowEquipmentMinDepthIn float64 = 6.0

// CPSCCompressionMinFraction is the fraction by which loose fill compresses at
// least over time (CPSC 325, public handbook).
//
// Data sheet playground-surfacing-cpsc (public_handbook.compression_min_fraction), sheet source
// https://www.cpsc.gov/s3fs-public/324.pdf, read on https://www.cpsc.gov/s3fs-public/325.pdf,
// verified 2026-09-25.
const CPSCCompressionMinFraction float64 = 0.25

// CPSCRules are the CPSC rules in millimetres.
type CPSCRules struct {
	// AllDirectionsMM is the extent in all directions, mm.
	AllDirectionsMM float64
	// SwingMultiple is the swing multiple of the top bar.
	SwingMultiple float64
	// TireExtraMM is the tire swing extra, mm.
	TireExtraMM float64
	// MaxEquipmentMM is the tallest equipment, mm.
	MaxEquipmentMM float64
	// LowEquipmentMM is the low equipment threshold, mm.
	LowEquipmentMM float64
	// MaintainedMM is the maintained depth, mm.
	MaintainedMM float64
	// LowMaintainedMM is the maintained depth under low equipment, mm.
	LowMaintainedMM float64
	// Compression is the minimum compression fraction.
	Compression float64
}

// MaxDepth is the published maximum depth by texture, mm (Penn State).
type MaxDepth struct {
	// Coarse is for coarse mulch.
	Coarse float64
	// Fine is for fine mulch.
	Fine float64
}

// Rules are the rules of the mulch calculator.
type Rules struct {
	// CuFtPerCuYd is the number of cubic feet per cubic yard.
	CuFtPerCuYd float64
	// CPSC are the CPSC rules.
	CPSC CPSCRules
	// MaxDepth are the maximum depths.
	MaxDepth MaxDepth
}

// NewRules builds the mulch rules: Penn State maximum depths, CPSC home
// handbook (324) rules with the public handbook (325) compression, cubic feet
// per cubic yard.
func NewRules() Rules {
	var psu Publisher
	for _, p := range Publishers {
		if p.ID == "psu" {
			psu = p
			break
		}
	}
	upper := func(v *[2]float64) float64 {
		if v == nil {
			return math.NaN()
		}
		return math.Max(v[0], v[1])
	}
	return Rules{
		CuFtPerCuYd: soil.CubicFeetPerCubicYard,
		CPSC: CPSCRules{
			AllDirectionsMM: CPSCAllDirectionsFt * calculiva.Foot,
			SwingMultiple:   CPSCSwingMultiple,
			TireExtraMM:     CPSCTireSwingExtraFt * calculiva.Foot,
			MaxEquipmentMM:  CPSCWoodMulchMaxEquipmentFt * calculiva.Foot,
			LowEquipmentMM:  CPSCLowEquipmentFt * calculiva.Foot,
			MaintainedMM:    CPSCMaintainedDepthIn * calculiva.Inch,
			LowMaintainedMM: CPSCLowEquipmentMinDepthIn * calculiva.Inch,
			Compression:     CPSCCompressionMinFraction,
		},
		MaxDepth: MaxDepth{Coarse: upper(psu.DepthIn) * calculiva.Inch, Fine: upper(psu.FineDepthIn) * calculiva.Inch},
	}
}

// Minimum is a published minimum: trunk gap or ring radius.
type Minimum struct {
	// ID is the publisher id.
	ID string
	// Publisher is the publisher name.
	Publisher string
	// Size is the tree size, for ring radii; empty for trunk gaps.
	Size string
	// MinMM is the minimum, mm.
	MinMM float64
}

// RingGuidance holds the trunk gaps and minimum ring radii.
type RingGuidance struct {
	// Gaps are the bare gaps at the trunk, by publisher.
	Gaps []Minimum
	// Radii are the minimum ring radii by tree size (NCUFC).
	Radii []Minimum
}

// NewRingGuidance returns the trunk gaps and minimum ring radii, publisher by
// publisher; a range counts by its minimum. Penn State publishes no gap figure
// and does not appear.
func NewRingGuidance() RingGuidance {
	g := RingGuidance{Gaps: []Minimum{}, Radii: []Minimum{}}
	for _, p := range Publishers {
		if p.TrunkGapIn != nil {
			g.Gaps = append(g.Gaps, Minimum{ID: p.ID, Publisher: p.Name, MinMM: math.Min(p.TrunkGapIn[0], p.TrunkGapIn[1]) * calculiva.Inch})
		}
	}
	for _, p := range Publishers {
		if p.RingRadiusByTreeFt != nil {
			for _, r := range p.RingRadiusByTreeFt {
				g.Radii = append(g.Radii, Minimum{ID: p.ID, Publisher: p.Name, Size: r.Size, MinMM: r.Ft * calculiva.Foot})
			}
			break
		}
	}
	return g
}

// Checked is a published minimum and whether the ring meets it.
type Checked struct {
	// Minimum is the minimum.
	Minimum Minimum
	// OK tells whether it is met.
	OK bool
}

// RingCheck is the result of [CheckRing].
type RingCheck struct {
	// Gaps are the trunk gaps.
	Gaps []Checked
	// Radii are the ring radii.
	Radii []Checked
}

// CheckRing checks a tree ring (radius r and bare gap gap, mm) against every
// published gap and ring radius.
func CheckRing(r, gap float64, guidance RingGuidance) RingCheck {
	k := RingCheck{Gaps: make([]Checked, 0, len(guidance.Gaps)), Radii: make([]Checked, 0, len(guidance.Radii))}
	for _, g := range guidance.Gaps {
		k.Gaps = append(k.Gaps, Checked{Minimum: g, OK: gap >= g.MinMM-eps})
	}
	for _, g := range guidance.Radii {
		k.Radii = append(k.Radii, Checked{Minimum: g, OK: r >= g.MinMM-eps})
	}
	return k
}

// Texture is coarse or fine mulch.
type Texture int

const (
	// Coarse is coarse mulch (bark, chips).
	Coarse Texture = iota
	// Fine is fine mulch (shredded).
	Fine
)

// Kind is the kind of a mulch area, lengths in mm: [Bed], [Ring], [KnownArea],
// [Play] or [Swing].
type Kind interface {
	isKind()
}

// Bed is a rectangular bed.
type Bed struct {
	// L is the length.
	L float64
	// W is the width.
	W float64
	// Count is the number of copies (1 to 100).
	Count float64
}

// Ring is a ring around a tree: outer radius, trunk diameter, bare gap at the
// trunk.
type Ring struct {
	// R is the outer radius.
	R float64
	// Trunk is the trunk diameter.
	Trunk float64
	// Gap is the bare gap.
	Gap float64
	// Count is the number of copies (1 to 100).
	Count float64
}

// KnownArea is a known area, mm².
type KnownArea struct {
	// Area is the area in mm².
	Area float64
}

// Play is a play structure footprint; the depth comes from CPSC.
type Play struct {
	// L is the length.
	L float64
	// W is the width.
	W float64
	// Height is the equipment height.
	Height float64
}

// Swing is a to-fro swing; the depth and extent come from CPSC.
type Swing struct {
	// Beam is the top bar length.
	Beam float64
	// Height is the top bar height.
	Height float64
}

func (Bed) isKind()       {}
func (Ring) isKind()      {}
func (KnownArea) isKind() {}
func (Play) isKind()      {}
func (Swing) isKind()     {}

// Area is one mulch area.
type Area struct {
	// ID is the area id; it prefixes error fields.
	ID string
	// Kind holds the kind and dimensions.
	Kind Kind
	// Target is the target depth, mm (garden kinds only).
	Target float64
	// Existing is the depth already in place, mm.
	Existing float64
}

// Input is the input of [NewPlan]. The zero value of every field but Areas is
// the calculator default.
type Input struct {
	// Areas holds one to eight areas.
	Areas []Area
	// Texture is the mulch texture.
	Texture Texture
	// BagSize is the bag size, 0 for none.
	BagSize float64
	// BagUnit is the bag unit (cubic feet or litres).
	BagUnit soil.BagUnit
	// BagPrice is the bag price.
	BagPrice float64
	// BulkPrice is the bulk price per cubic yard.
	BulkPrice float64
	// Delivery is the delivery charge.
	Delivery float64
	// Increment is the sale increment, cu yd, 0 for none.
	Increment float64
	// Extra is the extra, percent.
	Extra float64
	// BedVolume is the bed volume, cu ft, 0 for none.
	BedVolume float64
	// Wheelbarrow is the wheelbarrow volume, cu ft, 0 for none.
	Wheelbarrow float64
}

// AreaPlan is one area's result. A nil pointer means the value does not apply
// to the kind of area.
type AreaPlan struct {
	// ID is the area id.
	ID string
	// Kind is the kind.
	Kind Kind
	// Count is the number of copies.
	Count float64
	// Target is the depth to reach, mm (CPSC initial fill for play and swing).
	Target float64
	// Existing is the depth already in place, mm.
	Existing float64
	// AreaMM2 is the area of one copy, mm².
	AreaMM2 float64
	// SqFt is the area of one copy, sq ft.
	SqFt float64
	// AddedDepth is the depth added, mm.
	AddedDepth float64
	// Volume is the volume, mm³.
	Volume float64
	// CuFt is the volume, cu ft.
	CuFt float64
	// CuYd is the volume, cu yd.
	CuYd float64
	// InnerMM is the inner bare radius of a ring, mm.
	InnerMM *float64
	// MaintainedMM is the CPSC maintained depth, mm.
	MaintainedMM *float64
	// InitialMM is the CPSC initial fill, mm.
	InitialMM *float64
	// ExtentL is the surfaced length, mm.
	ExtentL *float64
	// ExtentW is the surfaced width, mm.
	ExtentW *float64
	// FrontMM is the swing surfacing in front and behind, mm.
	FrontMM *float64
}

// Totals are the totals.
type Totals struct {
	// Volume is in mm³.
	Volume float64
	// CuFt is in cu ft.
	CuFt float64
	// CuYd is in cu yd.
	CuYd float64
	// OrderedCuFt is the volume with the extra, cu ft.
	OrderedCuFt float64
	// OrderedCuYd is the volume with the extra, cu yd.
	OrderedCuYd float64
	// OrderedVolume is the volume with the extra, mm³.
	OrderedVolume float64
	// BulkCuYd is the bulk volume after the increment, cu yd.
	BulkCuYd float64
	// BulkVolume is the bulk volume, mm³.
	BulkVolume float64
}

// Buy holds the buying options. A nil pointer means the value is absent: the
// input it depends on was not entered.
type Buy struct {
	// BagCuFt is the bag volume, cu ft.
	BagCuFt *float64
	// Bags is the number of bags.
	Bags *float64
	// BagCost is the bag cost.
	BagCost *float64
	// BulkCuYd is the bulk volume, cu yd.
	BulkCuYd float64
	// BulkSurplusCuFt is the bulk surplus, cu ft.
	BulkSurplusCuFt float64
	// BulkCost is the bulk cost with delivery.
	BulkCost *float64
	// Cheaper is the cheaper option, empty when a price is missing.
	Cheaper soil.Cheaper
	// BagPerCuFt is the bag price per cu ft.
	BagPerCuFt *float64
	// BulkPerCuFt is the bulk price per cu ft.
	BulkPerCuFt *float64
	// BreakEvenCuFt is the break-even volume, cu ft.
	BreakEvenCuFt *float64
}

// WarningKind is the short kind of a warning, as used on the calculator page.
type WarningKind string

const (
	// OverMax flags a target deeper than the published maximum for the texture.
	OverMax WarningKind = "over-max"
	// AlreadyDeep flags existing mulch that already reaches the target.
	AlreadyDeep WarningKind = "already-deep"
	// DeepRefill flags existing mulch deeper than the target.
	DeepRefill WarningKind = "deep-refill"
	// Increment flags a bulk increment that adds volume.
	Increment WarningKind = "increment"
)

// Warning is a planning warning. The fields that do not belong to its kind are
// left at zero.
type Warning struct {
	// Kind is the kind of warning.
	Kind WarningKind
	// Area is the area id, when the warning belongs to an area.
	Area string
	// MaxMM is the maximum, mm (OverMax).
	MaxMM float64
	// Existing is the existing depth, mm (AlreadyDeep, DeepRefill).
	Existing float64
	// Target is the target depth, mm (OverMax, AlreadyDeep, DeepRefill).
	Target float64
	// SurplusCuFt is the surplus, cu ft (Increment).
	SurplusCuFt float64
}

// Plan is the result of [NewPlan].
type Plan struct {
	// Areas are the areas.
	Areas []AreaPlan
	// Totals are the totals.
	Totals Totals
	// Buy holds the buying options.
	Buy Buy
	// Trips is the number of truck trips by bed volume, nil without one.
	Trips *float64
	// WheelbarrowLoads is the number of wheelbarrow loads, nil without a
	// wheelbarrow volume.
	WheelbarrowLoads *float64
	// Warnings are the warnings.
	Warnings []Warning
}

func ptr(v float64) *float64 { return &v }

// NewPlan plans mulch: volume per area (CPSC depth and extent for play areas),
// bags, bulk, cost and loads.
func NewPlan(c Input, rules Rules) (*Plan, error) {
	if len(c.Areas) == 0 || len(c.Areas) > 8 {
		return nil, num.NewError("Tick one to eight areas.", "areas")
	}
	var bagMin, bagMax float64
	var bagUnit string
	switch c.BagUnit {
	case soil.CubicFeet:
		bagMin, bagMax, bagUnit = 0.05, 10.0, "cu ft"
	case soil.Liters:
		bagMin, bagMax, bagUnit = 1.0, 300.0, "liters"
	default:
		return nil, num.NewError("Choose a bag unit.", "bag-unit")
	}
	if c.BagSize != 0.0 {
		if err := num.Range(c.BagSize, bagMin, bagMax, fmt.Sprintf("Enter a bag size from %s to %s %s, or 0.", num.JSNum(bagMin), num.JSNum(bagMax), bagUnit), "bag"); err != nil {
			return nil, err
		}
	}
	if err := num.Range(c.BagPrice, 0.0, 100000.0, "Enter a bag price from 0 to 100,000 USD.", "bag-price"); err != nil {
		return nil, err
	}
	if err := num.Range(c.BulkPrice, 0.0, 100000.0, "Enter a bulk price from 0 to 100,000 USD.", "bulk-price"); err != nil {
		return nil, err
	}
	if err := num.Range(c.Delivery, 0.0, 10000.0, "Enter a delivery charge from 0 to 10,000 USD.", "delivery"); err != nil {
		return nil, err
	}
	if c.Increment != 0.0 {
		if err := num.Range(c.Increment, 0.1, 10.0, "Enter a sale increment from 0.1 to 10 cu yd, or 0.", "increment"); err != nil {
			return nil, err
		}
	}
	if err := num.Range(c.Extra, 0.0, 100.0, "Enter an extra from 0 to 100 %.", "extra"); err != nil {
		return nil, err
	}
	if c.BedVolume != 0.0 {
		if err := num.Range(c.BedVolume, 1.0, 200.0, "Enter a bed volume from 1 to 200 cu ft, or 0.", "bed-volume"); err != nil {
			return nil, err
		}
	}
	if c.Wheelbarrow != 0.0 {
		if err := num.Range(c.Wheelbarrow, 1.0, 20.0, "Enter a wheelbarrow volume from 1 to 20 cu ft, or 0.", "wheelbarrow"); err != nil {
			return nil, err
		}
	}
	perYd := rules.CuFtPerCuYd
	p := rules.CPSC
	maxMM := rules.MaxDepth.Coarse
	if c.Texture == Fine {
		maxMM = rules.MaxDepth.Fine
	}
	warnings := []Warning{}
	areas := make([]AreaPlan, 0, len(c.Areas))
	for _, a := range c.Areas {
		f := func(k string) string { return a.ID + "-" + k }
		length := func(v float64, k, label string) error {
			return num.Range(v, num.LenMin, num.LenMax, "Enter "+label+num.LenMsg, f(k))
		}
		copies := func(n float64) error {
			if n != math.Trunc(n) || !(n >= 1.0 && n <= 100.0) {
				return num.NewError("Enter a whole number of copies from 1 to 100.", f("count"))
			}
			return nil
		}
		// Play area: maintained depth by height (6 in under 4 ft, 9 in up to 8 ft, beyond that
		// outside the home handbook); initial fill = maintained / (1 - compression).
		cpscDepth := func(height float64) (maintained, initial float64, err error) {
			if err := num.Range(height, num.LenMin, num.LenMax, "Enter a height"+num.LenMsg, f("height")); err != nil {
				return 0, 0, err
			}
			if height > p.MaxEquipmentMM+eps {
				return 0, 0, num.NewError(
					fmt.Sprintf(
						"Enter a height up to %s ft (%s mm): taller equipment is outside the home handbook for wood mulch.",
						num.Locale(p.MaxEquipmentMM/calculiva.Foot, 1),
						num.Locale(p.MaxEquipmentMM, 1),
					),
					f("height"),
				)
			}
			maintained = p.MaintainedMM
			if height < p.LowEquipmentMM-eps {
				maintained = p.LowMaintainedMM
			}
			return maintained, maintained / (1.0 - p.Compression), nil
		}
		garden := false
		plan := AreaPlan{ID: a.ID, Kind: a.Kind, Count: 1.0, Target: a.Target, Existing: a.Existing}
		switch k := a.Kind.(type) {
		case Bed:
			garden = true
			areaMM2, err := soil.AreaOf(a.ID, soil.Rect{L: k.L, W: k.W})
			if err != nil {
				return nil, err
			}
			plan.AreaMM2 = areaMM2
			if err := copies(k.Count); err != nil {
				return nil, err
			}
			plan.Count = k.Count
		case KnownArea:
			garden = true
			areaMM2, err := soil.AreaOf(a.ID, soil.KnownArea{Area: k.Area})
			if err != nil {
				return nil, err
			}
			plan.AreaMM2 = areaMM2
		case Ring:
			garden = true
			// Trunk and gap at 0: the ring becomes a plain round bed.
			if err := length(k.R, "r", "a ring radius"); err != nil {
				return nil, err
			}
			if err := num.Range(k.Trunk, 0.0, num.LenMax, "Enter a trunk diameter from 0 to 60 m (0 to 196 ft).", f("trunk")); err != nil {
				return nil, err
			}
			if err := num.Range(k.Gap, 0.0, num.LenMax, "Enter a bare gap from 0 to 60 m (0 to 196 ft).", f("gap")); err != nil {
				return nil, err
			}
			inner := k.Trunk/2.0 + k.Gap
			if inner >= k.R-eps {
				return nil, num.NewError("The trunk and its bare gap must fit inside the ring radius.", f("r"))
			}
			plan.AreaMM2 = math.Pi * (k.R*k.R - inner*inner)
			if err := copies(k.Count); err != nil {
				return nil, err
			}
			plan.Count = k.Count
			plan.InnerMM = ptr(inner)
		case Play:
			// CPSC surface: the footprint plus 6 ft on every side.
			maintained, initial, err := cpscDepth(k.Height)
			if err != nil {
				return nil, err
			}
			if err := length(k.L, "l", "a length"); err != nil {
				return nil, err
			}
			extentL := k.L + 2.0*p.AllDirectionsMM
			if err := length(k.W, "w", "a width"); err != nil {
				return nil, err
			}
			extentW := k.W + 2.0*p.AllDirectionsMM
			plan.AreaMM2 = extentL * extentW
			plan.Target = initial
			plan.MaintainedMM = ptr(maintained)
			plan.InitialMM = ptr(initial)
			plan.ExtentL = ptr(extentL)
			plan.ExtentW = ptr(extentW)
		case Swing:
			// Swing: twice the top bar height in front and behind, never under 6 ft all round.
			maintained, initial, err := cpscDepth(k.Height)
			if err != nil {
				return nil, err
			}
			if err := length(k.Beam, "beam", "a top bar length"); err != nil {
				return nil, err
			}
			extentL := k.Beam + 2.0*p.AllDirectionsMM
			front := math.Max(p.SwingMultiple*k.Height, p.AllDirectionsMM)
			extentW := 2.0 * front
			plan.AreaMM2 = extentL * extentW
			plan.Target = initial
			plan.MaintainedMM = ptr(maintained)
			plan.InitialMM = ptr(initial)
			plan.ExtentL = ptr(extentL)
			plan.ExtentW = ptr(extentW)
			plan.FrontMM = ptr(front)
		default:
			return nil, num.NewError("Choose a kind of area.", f("kind"))
		}
		if garden {
			if err := num.Range(a.Target, depthMin, depthMax, depthMsg, f("target")); err != nil {
				return nil, err
			}
		}
		if err := num.Range(a.Existing, 0.0, depthMax, existingMsg, f("existing")); err != nil {
			return nil, err
		}
		target := plan.Target
		plan.AddedDepth = math.Max(target-a.Existing, 0.0)
		plan.Volume = plan.AreaMM2 * plan.AddedDepth * plan.Count
		if garden && target > maxMM+eps {
			warnings = append(warnings, Warning{Kind: OverMax, Area: a.ID, MaxMM: maxMM, Target: target})
		}
		if a.Existing >= target-eps {
			warnings = append(warnings, Warning{Kind: AlreadyDeep, Area: a.ID, Existing: a.Existing, Target: target})
		}
		if garden && a.Existing > target+eps {
			warnings = append(warnings, Warning{Kind: DeepRefill, Area: a.ID, Existing: a.Existing, Target: target})
		}
		plan.SqFt = plan.AreaMM2 / soil.SqFt
		plan.CuFt = plan.Volume / soil.CuFt
		plan.CuYd = plan.Volume / soil.CuFt / perYd
		areas = append(areas, plan)
	}
	volume := 0.0
	for _, a := range areas {
		volume += a.Volume
	}
	cuft := volume / soil.CuFt
	cuyd := cuft / perYd
	orderedCuFt := cuft * (1.0 + c.Extra/100.0)
	orderedCuYd := orderedCuFt / perYd
	// Bags, bulk, break-even and hauling: the soil formulas, by volume only.
	var bc, bags, bagCost *float64
	if c.BagSize != 0.0 {
		bc = ptr(soil.BagCuFt(c.BagSize, c.BagUnit, math.NaN()))
		bags = ptr(num.CeilRel(orderedCuFt / *bc))
		if c.BagPrice != 0.0 {
			bagCost = ptr(*bags * c.BagPrice)
		}
	}
	bulkCuYd := orderedCuYd
	if c.Increment != 0.0 {
		bulkCuYd = num.CeilRel(orderedCuYd/c.Increment) * c.Increment
	}
	bulkSurplusCuFt := (bulkCuYd - orderedCuYd) * perYd
	var bulkCost *float64
	if c.BulkPrice != 0.0 {
		bulkCost = ptr(bulkCuYd*c.BulkPrice + c.Delivery)
	}
	var cheaper soil.Cheaper
	if bagCost != nil && bulkCost != nil {
		if *bagCost <= *bulkCost {
			cheaper = soil.Bags
		} else {
			cheaper = soil.Bulk
		}
	}
	var bagPerCuFt, bulkPerCuFt, breakEvenCuFt *float64
	if bc != nil && *bc != 0.0 && c.BagPrice != 0.0 {
		bagPerCuFt = ptr(c.BagPrice / *bc)
	}
	if c.BulkPrice != 0.0 {
		bulkPerCuFt = ptr(c.BulkPrice / perYd)
	}
	if bagPerCuFt != nil && bulkPerCuFt != nil && *bagPerCuFt > *bulkPerCuFt {
		breakEvenCuFt = ptr(c.Delivery / (*bagPerCuFt - *bulkPerCuFt))
	}
	var trips, wheelbarrowLoads *float64
	if c.BedVolume != 0.0 {
		trips = ptr(num.CeilRel(orderedCuFt / c.BedVolume))
	}
	if c.Wheelbarrow != 0.0 {
		wheelbarrowLoads = ptr(num.CeilRel(orderedCuFt / c.Wheelbarrow))
	}
	if c.Increment != 0.0 && bulkSurplusCuFt > eps {
		warnings = append(warnings, Warning{Kind: Increment, SurplusCuFt: bulkSurplusCuFt})
	}
	return &Plan{
		Areas: areas,
		Totals: Totals{
			Volume:        volume,
			CuFt:          cuft,
			CuYd:          cuyd,
			OrderedCuFt:   orderedCuFt,
			OrderedCuYd:   orderedCuYd,
			OrderedVolume: orderedCuFt * soil.CuFt,
			BulkCuYd:      bulkCuYd,
			BulkVolume:    bulkCuYd * perYd * soil.CuFt,
		},
		Buy: Buy{
			BagCuFt:         bc,
			Bags:            bags,
			BagCost:         bagCost,
			BulkCuYd:        bulkCuYd,
			BulkSurplusCuFt: bulkSurplusCuFt,
			BulkCost:        bulkCost,
			Cheaper:         cheaper,
			BagPerCuFt:      bagPerCuFt,
			BulkPerCuFt:     bulkPerCuFt,
			BreakEvenCuFt:   breakEvenCuFt,
		},
		Trips:            trips,
		WheelbarrowLoads: wheelbarrowLoads,
		Warnings:         warnings,
	}, nil
}

// BagCoverage is the coverage of one bag size.
type BagCoverage struct {
	// CuFt is the bag volume, cu ft.
	CuFt float64
	// SqFt is the square feet covered at each depth asked.
	SqFt []float64
	// BagsPerCuYd is the number of bags per cubic yard.
	BagsPerCuYd float64
}

// Coverage returns the square feet covered by one bag at each depth (inches),
// and the bags per cubic yard.
func Coverage(bagCuFts, depthsIn []float64, rules Rules) []BagCoverage {
	inPerFt := calculiva.Foot / calculiva.Inch
	out := make([]BagCoverage, 0, len(bagCuFts))
	for _, cuft := range bagCuFts {
		sqft := make([]float64, 0, len(depthsIn))
		for _, d := range depthsIn {
			sqft = append(sqft, cuft*inPerFt/d)
		}
		out = append(out, BagCoverage{CuFt: cuft, SqFt: sqft, BagsPerCuYd: rules.CuFtPerCuYd / cuft})
	}
	return out
}
