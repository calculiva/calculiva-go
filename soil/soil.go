// Package soil estimates topsoil and fill dirt by area: volume, bags, bulk,
// weight and hauling.
//
// Lengths are millimetres; weights are pounds and short tons; sale volumes are
// cubic feet and cubic yards. No weight, sale unit or load rule lives in the
// code: they come from [Rules], built by [NewRules] from the constants below;
// the material weights and the wheelbarrow volume are entries of the user.
// The results are planning quantities: the supplier's quote and the vehicle's
// rated payload govern.
//
// Calculator page: https://calculiva.com/landscaping/soil-calculator/
package soil

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
const CuFt = SqFt * calculiva.Foot

const (
	cuIn          = calculiva.Inch * calculiva.Inch * calculiva.Inch
	litre float64 = 1e6
	sqM   float64 = 1e6
)

// PoundsPerShortTon is the number of pounds in a short ton.
//
// Data sheet soil-weights-and-sale-units (physical_basis.pounds_per_short_ton),
// source https://www.nist.gov/document/2026-hb-130-section-iv-b, verified 2026-09-25.
const PoundsPerShortTon float64 = 2000.0

// CubicFeetPerCubicYard is the number of cubic feet in a cubic yard.
//
// Data sheet soil-weights-and-sale-units (physical_basis.cubic_feet_per_cubic_yard),
// source https://www.nist.gov/document/2026-hb-130-section-iv-b, verified 2026-09-25.
const CubicFeetPerCubicYard float64 = 27.0

// DryQuartIn3 is the number of cubic inches in a U.S. dry quart (1/32 of the
// 2,150.42 in³ bushel).
//
// Data sheet soil-weights-and-sale-units (sale_units.dry_measure.dry_quart_in3),
// source https://www.nist.gov/document/2026-hb-130-section-iv-b
// (NIST Handbook 130, IV-A: https://www.nist.gov/document/2026-hb-130-section-iv), verified 2026-09-25.
const DryQuartIn3 float64 = 67.200625

// Weight is the loose and bank weight of a material, in pounds per cubic yard,
// and its load factor, as entered by the user (from the supplier's ticket or
// the reference of their choice).
type Weight struct {
	// ID is the row id.
	ID string
	// Label is the row label as published.
	Label string
	// Loose is the loose weight, lb per cubic yard.
	Loose float64
	// Bank is the bank weight, lb per cubic yard.
	Bank float64
	// LoadFactor is the load factor (loose to bank volume).
	LoadFactor float64
}

// Rules are the rules of the soil calculator.
type Rules struct {
	// PoundsPerTon is the number of pounds per short ton.
	PoundsPerTon float64
	// CuFtPerCuYd is the number of cubic feet per cubic yard.
	CuFtPerCuYd float64
	// DryQuartIn3 is the number of cubic inches per dry quart.
	DryQuartIn3 float64
	// Weights are the material weights, entered by the user (none by default).
	Weights []Weight
}

// NewRules builds the soil rules from the sourced constants of this package,
// with the material weights given by the user.
func NewRules(weights ...Weight) Rules {
	return Rules{
		PoundsPerTon: PoundsPerShortTon,
		CuFtPerCuYd:  CubicFeetPerCubicYard,
		DryQuartIn3:  DryQuartIn3,
		Weights:      append([]Weight(nil), weights...),
	}
}

func (r Rules) weight(id string) (Weight, bool) {
	for _, w := range r.Weights {
		if w.ID == id {
			return w, true
		}
	}
	return Weight{}, false
}

// Shape is the shape of an area: [Rect], [Circle], [Triangle] or [KnownArea].
// Lengths are mm; a KnownArea is already in mm².
type Shape interface {
	isShape()
}

// Rect is a rectangle, length by width.
type Rect struct {
	// L is the length.
	L float64
	// W is the width.
	W float64
}

// Circle is a circle of diameter D.
type Circle struct {
	// D is the diameter.
	D float64
}

// Triangle is a triangle, base by height.
type Triangle struct {
	// B is the base.
	B float64
	// H is the height.
	H float64
}

// KnownArea is a known area in mm².
type KnownArea struct {
	// Area is the area in mm².
	Area float64
}

func (Rect) isShape()      {}
func (Circle) isShape()    {}
func (Triangle) isShape()  {}
func (KnownArea) isShape() {}

// AreaOf returns the area of a shape in mm². id prefixes the field name of any
// error ("a1-l").
func AreaOf(id string, shape Shape) (float64, error) {
	f := func(k string) string { return id + "-" + k }
	length := func(v float64, k, label string) error {
		return num.Range(v, num.LenMin, num.LenMax, "Enter "+label+num.LenMsg, f(k))
	}
	switch s := shape.(type) {
	case Rect:
		if err := length(s.L, "l", "a length"); err != nil {
			return 0, err
		}
		if err := length(s.W, "w", "a width"); err != nil {
			return 0, err
		}
		return s.L * s.W, nil
	case Circle:
		if err := length(s.D, "d", "a diameter"); err != nil {
			return 0, err
		}
		return math.Pi * s.D * s.D / 4.0, nil
	case Triangle:
		if err := length(s.B, "b", "a base"); err != nil {
			return 0, err
		}
		if err := length(s.H, "h", "a height"); err != nil {
			return 0, err
		}
		return s.B * s.H / 2.0, nil
	case KnownArea:
		if err := num.Range(s.Area, 0.1*sqM, 10000.0*sqM, "Enter an area from 0.1 to 10,000 m² (about 1.1 to 107,639 sq ft).", f("area")); err != nil {
			return 0, err
		}
		return s.Area, nil
	default:
		return 0, num.NewError("Choose a shape.", f("shape"))
	}
}

// BagUnit is the unit in which a bag's volume is printed.
type BagUnit int

const (
	// CubicFeet is cubic feet.
	CubicFeet BagUnit = iota
	// DryQuarts is U.S. dry quarts.
	DryQuarts
	// Liters is litres.
	Liters
)

// bounds returns the accepted range of a bag size in this unit, and the unit's
// name as used in messages.
func (u BagUnit) bounds() (min, max float64, name string) {
	switch u {
	case DryQuarts:
		return 1.0, 400.0, "dry quarts"
	case Liters:
		return 1.0, 300.0, "liters"
	default:
		return 0.05, 10.0, "cu ft"
	}
}

// BagCuFt returns the volume of a bag in cubic feet. The dry quart comes from
// the rules (1/32 of the bushel); the litre and the cubic inch follow from the
// millimetre.
func BagCuFt(size float64, unit BagUnit, dryQuartIn3 float64) float64 {
	switch unit {
	case DryQuarts:
		return size * dryQuartIn3 * cuIn / CuFt
	case Liters:
		return size * litre / CuFt
	default:
		return size
	}
}

// Material is topsoil or fill dirt.
type Material int

const (
	// Topsoil is topsoil.
	Topsoil Material = iota
	// Fill is fill dirt.
	Fill
)

// ID returns "topsoil" or "fill".
func (m Material) ID() string {
	if m == Fill {
		return "fill"
	}
	return "topsoil"
}

// Area is one area to fill.
type Area struct {
	// ID is the area id; it prefixes error fields.
	ID string
	// Shape is the area's shape.
	Shape Shape
	// Depth is the depth in mm (3 to 1,500).
	Depth float64
	// Count is the number of identical copies (1 to 100).
	Count float64
	// Material tells which material fills the area.
	Material Material
}

// MaterialInput holds the bag and bulk details of one material.
type MaterialInput struct {
	// BagSize is the bag size, 0 for no bags.
	BagSize float64
	// BagUnit is the unit of BagSize.
	BagUnit BagUnit
	// BagPrice is the price of one bag, 0 if unknown.
	BagPrice float64
	// BulkPrice is the bulk price per cubic yard or per ton, 0 if unknown.
	BulkPrice float64
	// Weight is the id of a row of [Rules.Weights].
	Weight string
}

// BulkBy tells how the bulk price is quoted.
type BulkBy int

const (
	// Yard is a price per cubic yard.
	Yard BulkBy = iota
	// Ton is a price per short ton.
	Ton
)

// Input is the input of [NewPlan]. The zero value of every field but Areas is
// the calculator default.
type Input struct {
	// Areas holds one to eight areas.
	Areas []Area
	// Topsoil holds the topsoil details, required if an area uses topsoil.
	Topsoil *MaterialInput
	// Fill holds the fill details, required if an area uses fill.
	Fill *MaterialInput
	// BulkBy is the bulk price basis.
	BulkBy BulkBy
	// Delivery is the delivery charge.
	Delivery float64
	// Increment is the supplier's sale increment in cubic yards, 0 for none.
	Increment float64
	// Extra is the extra in percent.
	Extra float64
	// Payload is the vehicle payload in pounds, 0 for none.
	Payload float64
	// BedVolume is the bed volume in cubic feet, 0 for none.
	BedVolume float64
	// Wheelbarrow is the wheelbarrow volume in cubic feet, 0 for none.
	Wheelbarrow float64
}

// AreaPlan is one area's volume.
type AreaPlan struct {
	// ID is the area id.
	ID string
	// Material is the material.
	Material Material
	// Count is the number of copies.
	Count float64
	// Depth is the depth in mm.
	Depth float64
	// AreaMM2 is the area of one copy, mm².
	AreaMM2 float64
	// SqFt is the area of one copy, sq ft.
	SqFt float64
	// Volume is the volume of all copies, mm³.
	Volume float64
	// CuFt is the volume, cu ft.
	CuFt float64
	// CuYd is the volume, cu yd.
	CuYd float64
}

// Cheaper tells which option is cheaper. The zero value means that a price is
// missing.
type Cheaper string

const (
	// Bags means bags are cheaper (also on a tie).
	Bags Cheaper = "bags"
	// Bulk means bulk is cheaper.
	Bulk Cheaper = "bulk"
)

// Binding tells which limit sets the number of trips. The zero value means
// that no vehicle limit was entered.
type Binding string

const (
	// ByWeight means the payload weight binds.
	ByWeight Binding = "weight"
	// ByVolume means the bed volume binds.
	ByVolume Binding = "volume"
	// ByBoth means both give the same count.
	ByBoth Binding = "both"
)

// MaterialPlan is the order, cost and hauling for one material. A nil pointer
// means the value is absent: the input it depends on was not entered.
type MaterialPlan struct {
	// ID is the material.
	ID Material
	// Weight is the weight row used.
	Weight Weight
	// Volume is the volume needed, mm³.
	Volume float64
	// CuFt is the volume needed, cu ft.
	CuFt float64
	// CuYd is the volume needed, cu yd.
	CuYd float64
	// OrderedCuFt is the volume with the extra, cu ft.
	OrderedCuFt float64
	// OrderedCuYd is the volume with the extra, cu yd.
	OrderedCuYd float64
	// OrderedVolume is the volume with the extra, mm³.
	OrderedVolume float64
	// BagCuFt is the bag volume in cu ft.
	BagCuFt *float64
	// Bags is the number of bags to buy.
	Bags *float64
	// BagCost is the cost of the bags.
	BagCost *float64
	// BulkCuYd is the bulk volume after the sale increment, cu yd.
	BulkCuYd float64
	// BulkVolume is the bulk volume, mm³.
	BulkVolume float64
	// BulkSurplusCuFt is the bulk surplus over the order, cu ft.
	BulkSurplusCuFt float64
	// LooseLb is the loose weight of the bulk order, lb.
	LooseLb float64
	// BulkTons is the same, short tons.
	BulkTons float64
	// BulkCost is the bulk cost with delivery.
	BulkCost *float64
	// Cheaper is the cheaper option when both prices are known.
	Cheaper Cheaper
	// BagPerCuFt is the bag price per cu ft.
	BagPerCuFt *float64
	// BulkPerCuFt is the bulk price per cu ft.
	BulkPerCuFt *float64
	// BreakEvenCuFt is the volume where bulk with delivery equals bags, cu ft.
	BreakEvenCuFt *float64
	// OrderedLooseLb is the loose weight of the unrounded order, lb.
	OrderedLooseLb float64
	// BankLb is the bank weight of the unrounded order, lb.
	BankLb float64
	// SettledCuFt is the settled volume, cu ft.
	SettledCuFt float64
	// TripsByWeight is the number of trips by payload.
	TripsByWeight *float64
	// TripsByVolume is the number of trips by bed volume.
	TripsByVolume *float64
	// Trips is the number of trips needed.
	Trips *float64
	// Binding tells what sets the trips.
	Binding Binding
	// WheelbarrowLoads is the number of wheelbarrow loads.
	WheelbarrowLoads *float64
	// LbPerLoad is the pounds per wheelbarrow load.
	LbPerLoad *float64
}

// Totals are the totals over the materials.
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
	// BulkCuYd is the bulk volume, cu yd.
	BulkCuYd float64
	// LooseLb is the loose weight of the bulk, lb.
	LooseLb float64
	// Trips is the number of trips over the materials that have one, nil if
	// none has.
	Trips *float64
	// WheelbarrowLoads is the number of wheelbarrow loads, nil without a
	// wheelbarrow volume.
	WheelbarrowLoads *float64
}

// IncrementWarning tells that the bulk increment adds volume over the order.
type IncrementWarning struct {
	// Material is the material.
	Material Material
	// SurplusCuFt is the surplus in cu ft.
	SurplusCuFt float64
}

// Plan is the result of [NewPlan].
type Plan struct {
	// Areas are the areas.
	Areas []AreaPlan
	// Materials are the materials in use, topsoil first.
	Materials []MaterialPlan
	// Totals are the totals.
	Totals Totals
	// Warnings are the increment warnings.
	Warnings []IncrementWarning
}

func ptr(v float64) *float64 { return &v }

// NewPlan plans topsoil and fill: volume per area, bags, bulk, cost, weight and
// trips per material.
func NewPlan(c Input, rules Rules) (*Plan, error) {
	if len(c.Areas) == 0 || len(c.Areas) > 8 {
		return nil, num.NewError("Enter one to eight areas.", "areas")
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
	if c.Payload != 0.0 {
		if err := num.Range(c.Payload, 100.0, 20000.0, "Enter a payload from 100 to 20,000 lb, or 0.", "payload"); err != nil {
			return nil, err
		}
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
	lbTon := rules.PoundsPerTon
	areas := make([]AreaPlan, 0, len(c.Areas))
	for _, a := range c.Areas {
		f := func(k string) string { return a.ID + "-" + k }
		areaMM2, err := AreaOf(a.ID, a.Shape)
		if err != nil {
			return nil, err
		}
		if err := num.Range(a.Depth, 3.0, 1500.0, "Enter a depth from 1/8 in to 59 in (3 to 1,500 mm).", f("depth")); err != nil {
			return nil, err
		}
		if a.Count != math.Trunc(a.Count) || a.Count < 1.0 || a.Count > 100.0 {
			return nil, num.NewError("Enter a whole number of copies from 1 to 100.", f("count"))
		}
		volume := areaMM2 * a.Depth * a.Count
		areas = append(areas, AreaPlan{
			ID:       a.ID,
			Material: a.Material,
			Count:    a.Count,
			Depth:    a.Depth,
			AreaMM2:  areaMM2,
			SqFt:     areaMM2 / SqFt,
			Volume:   volume,
			CuFt:     volume / CuFt,
			CuYd:     volume / CuFt / perYd,
		})
	}
	materials := []MaterialPlan{}
	for _, entry := range []struct {
		id    Material
		input *MaterialInput
	}{{Topsoil, c.Topsoil}, {Fill, c.Fill}} {
		id, m := entry.id, entry.input
		used := false
		for _, a := range areas {
			if a.Material == id {
				used = true
				break
			}
		}
		if !used {
			continue
		}
		f := func(k string) string { return id.ID() + "-" + k }
		if m == nil {
			return nil, num.NewError("Enter the bag and bulk details of this material.", f("bag"))
		}
		if m.BagSize != 0.0 {
			min, max, u := m.BagUnit.bounds()
			if err := num.Range(m.BagSize, min, max, fmt.Sprintf("Enter a bag size from %s to %s %s, or 0.", num.JSNum(min), num.JSNum(max), u), f("bag")); err != nil {
				return nil, err
			}
		}
		if err := num.Range(m.BagPrice, 0.0, 100000.0, "Enter a bag price from 0 to 100,000 USD.", f("bag-price")); err != nil {
			return nil, err
		}
		if err := num.Range(m.BulkPrice, 0.0, 100000.0, "Enter a bulk price from 0 to 100,000 USD.", f("bulk-price")); err != nil {
			return nil, err
		}
		w, ok := rules.weight(m.Weight)
		if !ok {
			return nil, num.NewError("Choose a material weight.", f("weight"))
		}
		volume := 0.0
		for _, a := range areas {
			if a.Material == id {
				volume += a.Volume
			}
		}
		cuft := volume / CuFt
		cuyd := cuft / perYd
		orderedCuFt := cuft * (1.0 + c.Extra/100.0)
		orderedCuYd := orderedCuFt / perYd
		orderedVolume := orderedCuFt * CuFt
		// Bags are whole; bulk rounds up to the supplier's sale increment, never down.
		var bc, bags, bagCost *float64
		if m.BagSize != 0.0 {
			bc = ptr(BagCuFt(m.BagSize, m.BagUnit, rules.DryQuartIn3))
			bags = ptr(num.CeilRel(orderedCuFt / *bc))
			if m.BagPrice != 0.0 {
				bagCost = ptr(*bags * m.BagPrice)
			}
		}
		bulkCuYd := orderedCuYd
		if c.Increment != 0.0 {
			bulkCuYd = num.CeilRel(orderedCuYd/c.Increment) * c.Increment
		}
		bulkSurplusCuFt := (bulkCuYd - orderedCuYd) * perYd
		looseLb := bulkCuYd * w.Loose
		bulkTons := looseLb / lbTon
		var bulkCost *float64
		if m.BulkPrice != 0.0 {
			if c.BulkBy == Yard {
				bulkCost = ptr(bulkCuYd*m.BulkPrice + c.Delivery)
			} else {
				bulkCost = ptr(bulkTons*m.BulkPrice + c.Delivery)
			}
		}
		var cheaper Cheaper
		if bagCost != nil && bulkCost != nil {
			if *bagCost <= *bulkCost {
				cheaper = Bags
			} else {
				cheaper = Bulk
			}
		}
		// Continuous break-even: the volume where bulk with delivery equals bags, unrounded.
		var bagPerCuFt, bulkPerCuFt, breakEvenCuFt *float64
		if bc != nil && *bc != 0.0 && m.BagPrice != 0.0 {
			bagPerCuFt = ptr(m.BagPrice / *bc)
		}
		if m.BulkPrice != 0.0 {
			if c.BulkBy == Yard {
				bulkPerCuFt = ptr(m.BulkPrice / perYd)
			} else {
				bulkPerCuFt = ptr(m.BulkPrice * w.Loose / lbTon / perYd)
			}
		}
		if bagPerCuFt != nil && bulkPerCuFt != nil && *bagPerCuFt > *bulkPerCuFt {
			breakEvenCuFt = ptr(c.Delivery / (*bagPerCuFt - *bulkPerCuFt))
		}
		// Hauling: weight is read on the unrounded order (what one loads oneself).
		orderedLooseLb := orderedCuYd * w.Loose
		bankLb := orderedCuYd * w.Bank
		settledCuFt := orderedCuFt * w.LoadFactor
		var tripsByWeight, tripsByVolume, trips *float64
		if c.Payload != 0.0 {
			tripsByWeight = ptr(num.CeilRel(orderedLooseLb / c.Payload))
		}
		if c.BedVolume != 0.0 {
			tripsByVolume = ptr(num.CeilRel(orderedCuFt / c.BedVolume))
		}
		var binding Binding
		switch {
		case tripsByWeight != nil && tripsByVolume != nil:
			a, b := *tripsByWeight, *tripsByVolume
			trips = ptr(math.Max(a, b))
			switch {
			case a > b:
				binding = ByWeight
			case b > a:
				binding = ByVolume
			default:
				binding = ByBoth
			}
		case tripsByWeight != nil:
			trips = ptr(*tripsByWeight)
			binding = ByWeight
		case tripsByVolume != nil:
			trips = ptr(*tripsByVolume)
			binding = ByVolume
		}
		var wheelbarrowLoads, lbPerLoad *float64
		if c.Wheelbarrow != 0.0 {
			wheelbarrowLoads = ptr(num.CeilRel(orderedCuFt / c.Wheelbarrow))
			lbPerLoad = ptr(c.Wheelbarrow / perYd * w.Loose)
		}
		materials = append(materials, MaterialPlan{
			ID:               id,
			Weight:           w,
			Volume:           volume,
			CuFt:             cuft,
			CuYd:             cuyd,
			OrderedCuFt:      orderedCuFt,
			OrderedCuYd:      orderedCuYd,
			OrderedVolume:    orderedVolume,
			BagCuFt:          bc,
			Bags:             bags,
			BagCost:          bagCost,
			BulkCuYd:         bulkCuYd,
			BulkVolume:       bulkCuYd * perYd * CuFt,
			BulkSurplusCuFt:  bulkSurplusCuFt,
			LooseLb:          looseLb,
			BulkTons:         bulkTons,
			BulkCost:         bulkCost,
			Cheaper:          cheaper,
			BagPerCuFt:       bagPerCuFt,
			BulkPerCuFt:      bulkPerCuFt,
			BreakEvenCuFt:    breakEvenCuFt,
			OrderedLooseLb:   orderedLooseLb,
			BankLb:           bankLb,
			SettledCuFt:      settledCuFt,
			TripsByWeight:    tripsByWeight,
			TripsByVolume:    tripsByVolume,
			Trips:            trips,
			Binding:          binding,
			WheelbarrowLoads: wheelbarrowLoads,
			LbPerLoad:        lbPerLoad,
		})
	}
	var totals Totals
	withTrips, trips, loads := false, 0.0, 0.0
	for _, m := range materials {
		totals.Volume += m.Volume
		totals.CuFt += m.CuFt
		totals.CuYd += m.CuYd
		totals.OrderedCuFt += m.OrderedCuFt
		totals.OrderedCuYd += m.OrderedCuYd
		totals.OrderedVolume += m.OrderedVolume
		totals.BulkCuYd += m.BulkCuYd
		totals.LooseLb += m.LooseLb
		if m.Trips != nil {
			withTrips = true
			trips += *m.Trips
		}
		if m.WheelbarrowLoads != nil {
			loads += *m.WheelbarrowLoads
		}
	}
	if withTrips {
		totals.Trips = ptr(trips)
	}
	if c.Wheelbarrow != 0.0 {
		totals.WheelbarrowLoads = ptr(loads)
	}
	warnings := []IncrementWarning{}
	for _, m := range materials {
		if c.Increment != 0.0 && m.BulkSurplusCuFt > eps {
			warnings = append(warnings, IncrementWarning{Material: m.ID, SurplusCuFt: m.BulkSurplusCuFt})
		}
	}
	return &Plan{Areas: areas, Materials: materials, Totals: totals, Warnings: warnings}, nil
}

// CoverageRow is the area covered at one depth.
type CoverageRow struct {
	// Inches is the depth in inches.
	Inches float64
	// SqFtPerCuYd is the square feet per cubic yard.
	SqFtPerCuYd float64
	// SqFtPerTon is the square feet per short ton, at the loose weight.
	SqFtPerTon float64
	// SqFtPerCuFt is the square feet per cubic foot.
	SqFtPerCuFt float64
}

// Coverage returns the area covered at each depth: per cubic yard, per ton
// (loose weight of weightID) and per cubic foot.
func Coverage(depthsIn []float64, rules Rules, weightID string) ([]CoverageRow, error) {
	w, ok := rules.weight(weightID)
	if !ok {
		return nil, num.NewError("Choose a material weight.", "weight")
	}
	inPerFt := calculiva.Foot / calculiva.Inch
	out := make([]CoverageRow, 0, len(depthsIn))
	for _, inches := range depthsIn {
		sqftPerCuYd := rules.CuFtPerCuYd * inPerFt / inches
		out = append(out, CoverageRow{
			Inches:      inches,
			SqFtPerCuYd: sqftPerCuYd,
			SqFtPerTon:  sqftPerCuYd * rules.PoundsPerTon / w.Loose,
			SqFtPerCuFt: inPerFt / inches,
		})
	}
	return out, nil
}
