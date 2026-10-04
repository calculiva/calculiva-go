package mulch

import (
	"errors"
	"math"
	"slices"
	"testing"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/soil"
)

const (
	in = 25.4
	ft = 304.8
)

func near(t *testing.T, a, b, tol float64) {
	t.Helper()
	if !(math.Abs(a-b) <= tol) {
		t.Errorf("%v vs %v", a, b)
	}
}

// some checks that an optional value is present and equal to want.
func some(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Errorf("got none, want %v", want)
	} else if *got != want {
		t.Errorf("got %v, want %v", *got, want)
	}
}

// none checks that an optional value is absent.
func none(t *testing.T, got *float64) {
	t.Helper()
	if got != nil {
		t.Errorf("got %v, want none", *got)
	}
}

// val unwraps an optional value.
func val(t *testing.T, got *float64) float64 {
	t.Helper()
	if got == nil {
		t.Fatal("got none, want a value")
	}
	return *got
}

func area(id string, kind Kind, target, existing float64) Area {
	return Area{ID: id, Kind: kind, Target: target, Existing: existing}
}

func a1() Area {
	return area("a1", Bed{L: 40.0 * ft, W: 3.0 * ft, Count: 2.0}, 3.0*in, 1.0*in)
}

func a2() Area {
	return area("a2", Ring{R: 3.0 * ft, Trunk: 10.0 * in, Gap: 3.0 * in, Count: 3.0}, 3.0*in, 0.0)
}

func a3() Area {
	return area("a3", KnownArea{Area: 150.0 * soil.SqFt}, 3.0*in, 0.0)
}

func a4(height float64) Area {
	return area("a4", Play{L: 8.0 * ft, W: 10.0 * ft, Height: height}, 0.0, 0.0)
}

func a5(height float64) Area {
	return area("a5", Swing{Beam: 10.0 * ft, Height: height}, 0.0, 0.0)
}

func job() Input {
	return Input{Areas: []Area{a1(), a2(), a3()}, BagSize: 2.0, BagUnit: soil.CubicFeet, Wheelbarrow: 6.0}
}

func one(a Area) Input {
	return Input{Areas: []Area{a}, BagSize: 2.0}
}

func plan(t *testing.T, c Input) *Plan {
	t.Helper()
	r, err := NewPlan(c, NewRules())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return r
}

func kinds(p *Plan) []WarningKind {
	out := []WarningKind{}
	for _, w := range p.Warnings {
		out = append(out, w.Kind)
	}
	return out
}

func wantKinds(t *testing.T, p *Plan, want ...WarningKind) {
	t.Helper()
	if got := kinds(p); !slices.Equal(got, want) {
		t.Errorf("warnings = %v, want %v", got, want)
	}
}

func fieldOf(t *testing.T, c Input) string {
	t.Helper()
	_, err := NewPlan(c, NewRules())
	var ie *calculiva.InputError
	if !errors.As(err, &ie) {
		t.Fatalf("got %v, want an input error", err)
	}
	return ie.Field
}

func TestRulesComeFromTheDataFiles(t *testing.T) {
	rules := NewRules()
	if rules.CuFtPerCuYd != 27.0 {
		t.Errorf("cu ft per cu yd = %v, want 27", rules.CuFtPerCuYd)
	}
	near(t, rules.MaxDepth.Coarse, 4.0*in, 1e-9)
	near(t, rules.MaxDepth.Fine, 2.0*in, 1e-9)
	p := rules.CPSC
	near(t, p.AllDirectionsMM, 6.0*ft, 1e-9)
	if p.SwingMultiple != 2.0 {
		t.Errorf("swing multiple = %v, want 2", p.SwingMultiple)
	}
	near(t, p.MaxEquipmentMM, 8.0*ft, 1e-9)
	near(t, p.LowEquipmentMM, 4.0*ft, 1e-9)
	near(t, p.MaintainedMM, 9.0*in, 1e-9)
	near(t, p.LowMaintainedMM, 6.0*in, 1e-9)
	if p.Compression != 0.25 {
		t.Errorf("compression = %v, want 0.25", p.Compression)
	}
	near(t, p.TireExtraMM, 6.0*ft, 1e-9)
	near(t, p.MaintainedMM/(1.0-p.Compression), CPSCInitialFillIn*in, 1e-9)
}

func TestDefaultExampleBedsTreeRingsTypedAreaAndTheTotal(t *testing.T) {
	r := plan(t, job())
	x1, x2, x3 := r.Areas[0], r.Areas[1], r.Areas[2]
	near(t, x1.SqFt*x1.Count, 240.0, 1e-9)
	near(t, x1.AddedDepth, 2.0*in, 1e-9)
	near(t, x1.CuFt, 40.0, 1e-9)
	near(t, val(t, x2.InnerMM), 8.0*in, 1e-9)
	near(t, x2.SqFt, 26.878, 1e-3)
	near(t, x2.SqFt*3.0, 80.634, 1e-3)
	near(t, x2.CuFt, 20.159, 1e-3)
	near(t, x3.CuFt, 37.5, 1e-9)
	near(t, r.Totals.CuFt, 97.659, 1e-3)
	near(t, r.Totals.CuYd, 3.617, 1e-3)
	near(t, r.Totals.OrderedCuFt, r.Totals.CuFt, 1e-12)
	if len(r.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", r.Warnings)
	}
}

func TestBagsInCubicFeetAndLitersWheelbarrowLoads(t *testing.T) {
	bags := func(size float64, unit soil.BagUnit) *float64 {
		c := job()
		c.BagSize, c.BagUnit = size, unit
		return plan(t, c).Buy.Bags
	}
	some(t, bags(2.0, soil.CubicFeet), 49.0)
	some(t, bags(1.5, soil.CubicFeet), 66.0)
	some(t, bags(3.0, soil.CubicFeet), 33.0)
	some(t, bags(50.0, soil.Liters), 56.0)
	near(t, soil.BagCuFt(50.0, soil.Liters, math.NaN()), 1.76573, 1e-5)
	none(t, bags(0.0, soil.CubicFeet))
	some(t, plan(t, job()).WheelbarrowLoads, 17.0)
	c := job()
	c.Wheelbarrow = 0.0
	none(t, plan(t, c).WheelbarrowLoads)
}

func TestPlayStructureCPSCExtentDepthByHeightInitialFill(t *testing.T) {
	p := func(h float64) AreaPlan { return plan(t, one(a4(h))).Areas[0] }
	tall := p(7.0 * ft)
	near(t, tall.SqFt, 440.0, 1e-9)
	near(t, val(t, tall.MaintainedMM), 9.0*in, 1e-9)
	near(t, val(t, tall.InitialMM), 12.0*in, 1e-9)
	near(t, tall.CuFt, 440.0, 1e-9)
	near(t, tall.CuYd, 16.296, 1e-3)
	low := p(3.0 * ft)
	near(t, val(t, low.MaintainedMM), 6.0*in, 1e-9)
	near(t, val(t, low.InitialMM), 8.0*in, 1e-9)
	near(t, low.CuFt, 293.333, 1e-3)
	near(t, val(t, p(4.0*ft).MaintainedMM), 9.0*in, 1e-9)
	near(t, val(t, p(8.0*ft).InitialMM), 12.0*in, 1e-9)
	if got := fieldOf(t, one(a4(9.0*ft))); got != "a4-height" {
		t.Errorf("field = %q, want a4-height", got)
	}
	filled := a4(7.0 * ft)
	filled.Existing = 5.0 * in
	topped := plan(t, one(filled)).Areas[0]
	near(t, topped.AddedDepth, 7.0*in, 1e-9)
}

func TestSwingTwiceTheTopBarFrontAndBackNeverUnder6Ft(t *testing.T) {
	s := plan(t, one(a5(8.0*ft))).Areas[0]
	near(t, val(t, s.ExtentL), 22.0*ft, 1e-9)
	near(t, val(t, s.ExtentW), 32.0*ft, 1e-9)
	near(t, s.SqFt, 704.0, 1e-9)
	near(t, val(t, s.InitialMM), 12.0*in, 1e-9)
	near(t, s.CuFt, 704.0, 1e-9)
	short := plan(t, one(a5(2.0*ft))).Areas[0]
	near(t, val(t, short.ExtentW), 12.0*ft, 1e-9)
	near(t, val(t, short.MaintainedMM), 6.0*in, 1e-9)
	if got := fieldOf(t, one(a5(9.0*ft))); got != "a5-height" {
		t.Errorf("field = %q, want a5-height", got)
	}
}

func TestCostBagsAgainstBulkByTheYardDeliveryAndBreakEven(t *testing.T) {
	c := job()
	c.BagPrice, c.BulkPrice, c.Delivery = 4.0, 30.0, 50.0
	b := plan(t, c).Buy
	some(t, b.Bags, 49.0)
	near(t, val(t, b.BagCost), 196.0, 1e-9)
	near(t, val(t, b.BulkCost), 158.51, 5e-3)
	if b.Cheaper != soil.Bulk {
		t.Errorf("cheaper = %q, want bulk", b.Cheaper)
	}
	near(t, val(t, b.BreakEvenCuFt), 56.25, 1e-9)
	c = job()
	c.BagPrice = 4.0
	noBulk := plan(t, c).Buy
	none(t, noBulk.BulkCost)
	if noBulk.Cheaper != "" {
		t.Errorf("cheaper = %q, want none", noBulk.Cheaper)
	}
	none(t, noBulk.BreakEvenCuFt)
	c = job()
	c.BagPrice, c.BulkPrice, c.Delivery = 1.0, 30.0, 50.0
	dear := plan(t, c).Buy
	if dear.Cheaper != soil.Bags {
		t.Errorf("cheaper = %q, want bags", dear.Cheaper)
	}
	none(t, dear.BreakEvenCuFt)
}

func TestExtraSaleIncrementAndTripsByBedVolume(t *testing.T) {
	c := job()
	c.Extra = 10.0
	x := plan(t, c)
	near(t, x.Totals.OrderedCuFt, 97.659*1.1, 1e-3)
	some(t, x.Buy.Bags, math.Ceil(97.659*1.1/2.0))
	c = job()
	c.Increment = 0.5
	r := plan(t, c)
	near(t, r.Buy.BulkCuYd, 4.0, 1e-9)
	near(t, r.Buy.BulkSurplusCuFt, (4.0-r.Totals.OrderedCuYd)*27.0, 1e-9)
	wantKinds(t, r, Increment)
	if len(r.Warnings) == 1 {
		near(t, r.Warnings[0].SurplusCuFt, r.Buy.BulkSurplusCuFt, 1e-12)
	}
	c = job()
	c.BedVolume = 30.0
	some(t, plan(t, c).Trips, 4.0)
	none(t, plan(t, job()).Trips)
}

func TestWarningsOverThePublishedMaximumAlreadyDeepDeepRefill(t *testing.T) {
	c := job()
	c.Texture = Fine
	fine := plan(t, c)
	type areaKind struct {
		area string
		kind WarningKind
	}
	got := []areaKind{}
	for _, w := range fine.Warnings {
		got = append(got, areaKind{w.Area, w.Kind})
	}
	if want := []areaKind{{"a1", OverMax}, {"a2", OverMax}, {"a3", OverMax}}; !slices.Equal(got, want) {
		t.Errorf("warnings = %v, want %v", got, want)
	}
	with := func(edit func(a *Area)) Input {
		a := a1()
		edit(&a)
		c := job()
		c.Areas = []Area{a}
		return c
	}
	deep := plan(t, with(func(a *Area) { a.Existing = 4.0 * in }))
	if deep.Areas[0].AddedDepth != 0.0 {
		t.Errorf("added depth = %v, want 0", deep.Areas[0].AddedDepth)
	}
	if deep.Areas[0].CuFt != 0.0 {
		t.Errorf("cu ft = %v, want 0", deep.Areas[0].CuFt)
	}
	wantKinds(t, deep, AlreadyDeep, DeepRefill)
	wantKinds(t, plan(t, with(func(a *Area) { a.Existing = 3.0 * in })), AlreadyDeep)
	wantKinds(t, plan(t, with(func(a *Area) { a.Target = 5.0 * in })), OverMax)
	filled := a4(7.0 * ft)
	filled.Existing = 13.0 * in
	wantKinds(t, plan(t, one(filled)), AlreadyDeep)
}

func TestRingCheckPublishersTrunkGapsAndNCUFCRadii(t *testing.T) {
	k := CheckRing(3.0*ft, 3.0*in, NewRingGuidance())
	type check struct {
		key string
		ok  bool
	}
	gaps := []check{}
	for _, g := range k.Gaps {
		gaps = append(gaps, check{g.Minimum.ID, g.OK})
		if g.Minimum.ID == "ucm" {
			near(t, g.Minimum.MinMM, 6.0*in, 1e-9)
		}
	}
	if want := []check{{"umd", true}, {"ucm", false}, {"ncufc", true}, {"casey", true}, {"maplegrove", true}}; !slices.Equal(gaps, want) {
		t.Errorf("gaps = %v, want %v", gaps, want)
	}
	radii := []check{}
	for _, g := range k.Radii {
		radii = append(radii, check{g.Minimum.Size, g.OK})
	}
	if want := []check{{"small", true}, {"medium", false}, {"large", false}}; !slices.Equal(radii, want) {
		t.Errorf("radii = %v, want %v", radii, want)
	}
}

func TestCoverageSquareFeetPerBagAndBagsPerCubicYard(t *testing.T) {
	// Synthetic bag sizes (1 and 3 cu ft), read on no product: the user enters the bag size.
	c := Coverage([]float64{1.0, 3.0}, []float64{1.0, 2.0, 3.0, 4.0}, NewRules())
	b1, b3 := c[0], c[1]
	near(t, b1.SqFt[0], 12.0, 1e-9)
	near(t, b1.SqFt[2], 4.0, 1e-9)
	near(t, b1.BagsPerCuYd, 27.0, 1e-9)
	near(t, b3.SqFt[0], 36.0, 1e-9)
	near(t, b3.SqFt[1], 18.0, 1e-9)
	near(t, b3.SqFt[2], 12.0, 1e-9)
	near(t, b3.SqFt[3], 9.0, 1e-9)
	near(t, b3.BagsPerCuYd, 9.0, 1e-9)
}

func TestInvalidInputNamesItsField(t *testing.T) {
	rules := NewRules()
	bad := func(c Input, field string) {
		t.Helper()
		_, err := NewPlan(c, rules)
		var ie *calculiva.InputError
		if !errors.As(err, &ie) {
			t.Errorf("%s: got %v, want an input error", field, err)
			return
		}
		if ie.Field != field {
			t.Errorf("field = %q, want %q", ie.Field, field)
		}
	}
	kind := func(a Area, k Kind) Area {
		a.Kind = k
		return a
	}
	bed := func(l, w, count float64) Area { return kind(a1(), Bed{L: l, W: w, Count: count}) }
	ring := func(r, trunk, gap float64) Area { return kind(a2(), Ring{R: r, Trunk: trunk, Gap: gap, Count: 3.0}) }
	depths := func(target, existing float64) Area {
		a := a1()
		a.Target, a.Existing = target, existing
		return a
	}
	opt := func(edit func(c *Input)) Input {
		c := one(a1())
		edit(&c)
		return c
	}
	bad(Input{Areas: []Area{}}, "areas")
	bad(Input{Areas: []Area{a1(), a1(), a1(), a1(), a1(), a1(), a1(), a1(), a1()}}, "areas")
	bad(one(bed(40.0, 3.0*ft, 2.0)), "a1-l")
	bad(one(bed(40.0*ft, 61000.0, 2.0)), "a1-w")
	bad(one(bed(40.0*ft, 3.0*ft, 0.0)), "a1-count")
	bad(one(bed(40.0*ft, 3.0*ft, 1.5)), "a1-count")
	bad(one(depths(2.0, 1.0*in)), "a1-target")
	bad(one(depths(601.0, 1.0*in)), "a1-target")
	bad(one(depths(3.0*in, -1.0)), "a1-existing")
	bad(one(depths(3.0*in, math.NaN())), "a1-existing")
	bad(one(ring(7.0*in, 10.0*in, 3.0*in)), "a2-r")
	bad(one(ring(8.0*in, 10.0*in, 3.0*in)), "a2-r")
	bad(one(ring(3.0*ft, -1.0, 3.0*in)), "a2-trunk")
	bad(one(ring(3.0*ft, 10.0*in, -1.0)), "a2-gap")
	bad(one(kind(a3(), KnownArea{Area: 0.05e6})), "a3-area")
	bad(one(kind(a3(), KnownArea{Area: 1e10 + 1e6})), "a3-area")
	bad(one(kind(a4(7.0*ft), Play{L: 10.0, W: 10.0 * ft, Height: 7.0 * ft})), "a4-l")
	bad(one(a4(10.0)), "a4-height")
	bad(one(kind(a5(8.0*ft), Swing{Beam: math.NaN(), Height: 8.0 * ft})), "a5-beam")
	bad(opt(func(c *Input) { c.BagUnit = soil.DryQuarts }), "bag-unit")
	bad(opt(func(c *Input) { c.BagSize = 11.0 }), "bag")
	bad(opt(func(c *Input) { c.BagSize, c.BagUnit = 301.0, soil.Liters }), "bag")
	bad(opt(func(c *Input) { c.BagPrice = -1.0 }), "bag-price")
	bad(opt(func(c *Input) { c.BulkPrice = 100001.0 }), "bulk-price")
	bad(opt(func(c *Input) { c.Delivery = 10001.0 }), "delivery")
	bad(opt(func(c *Input) { c.Increment = 0.05 }), "increment")
	bad(opt(func(c *Input) { c.Extra = 101.0 }), "extra")
	bad(opt(func(c *Input) { c.BedVolume = 0.5 }), "bed-volume")
	bad(opt(func(c *Input) { c.Wheelbarrow = 21.0 }), "wheelbarrow")
	if _, err := NewPlan(one(ring(3.0*ft, 10.0*in, 0.0)), rules); err != nil {
		t.Errorf("ring without a gap rejected: %v", err)
	}
	if _, err := NewPlan(one(depths(3.0, 1.0*in)), rules); err != nil {
		t.Errorf("3 mm target rejected: %v", err)
	}
}

func TestARingWithNoTrunkAndNoGapIsAPlainRoundBed(t *testing.T) {
	a := a2()
	a.Kind = Ring{R: 3.0 * ft, Trunk: 0.0, Gap: 0.0, Count: 1.0}
	r := plan(t, one(a))
	near(t, r.Areas[0].SqFt, math.Pi*9.0, 1e-6)
	near(t, r.Areas[0].CuFt, math.Pi*9.0*3.0/12.0, 1e-6)
}
