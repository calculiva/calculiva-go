package soil

import (
	"errors"
	"math"
	"slices"
	"testing"

	calculiva "github.com/calculiva/calculiva-go"
)

const (
	in = 25.4
	ft = 304.8
)

// Synthetic test values, read on no publisher or product: a 32-quart bag (one
// bushel) and two material weights. The user enters bag sizes and weights.
const synDryQuarts = 32.0

var synWeights = []Weight{
	{ID: "topsoil", Label: "synthetic topsoil", Loose: 2000.0, Bank: 2500.0, LoadFactor: 0.75},
	{ID: "earth-dry", Label: "synthetic fill", Loose: 2500.0, Bank: 3100.0, LoadFactor: 0.85},
}

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

func mat() MaterialInput {
	return MaterialInput{BagSize: 1.0, BagUnit: CubicFeet, BagPrice: 0.0, BulkPrice: 0.0, Weight: "topsoil"}
}

func area(id string, shape Shape, depth, count float64, material Material) Area {
	return Area{ID: id, Shape: shape, Depth: depth, Count: count, Material: material}
}

func areas() []Area {
	return []Area{
		area("a1", Rect{L: 8.0 * ft, W: 4.0 * ft}, 10.0*in, 3.0, Topsoil),
		area("a2", Circle{D: 6.0 * ft}, 8.0*in, 1.0, Topsoil),
		area("a3", KnownArea{Area: 1000.0 * SqFt}, 0.25*in, 1.0, Topsoil),
		area("a4", Triangle{B: 12.0 * ft, H: 10.0 * ft}, 4.0*in, 1.0, Fill),
	}
}

func job() Input {
	topsoil, fill := mat(), mat()
	fill.BagSize = 0.0
	fill.Weight = "earth-dry"
	return Input{Areas: areas(), Topsoil: &topsoil, Fill: &fill, Wheelbarrow: 6.0}
}

func withTopsoil(m MaterialInput) Input {
	c := job()
	c.Topsoil = &m
	return c
}

func plan(t *testing.T, c Input) *Plan {
	t.Helper()
	r, err := NewPlan(c, NewRules(synWeights...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return r
}

func materialIDs(r *Plan) []Material {
	ids := []Material{}
	for _, m := range r.Materials {
		ids = append(ids, m.ID)
	}
	return ids
}

func areaOf(t *testing.T, id string, shape Shape) float64 {
	t.Helper()
	a, err := AreaOf(id, shape)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return a
}

func TestDefaultExampleEveryAreaBothMaterialsAndTheTotal(t *testing.T) {
	r := plan(t, job())
	a1, a2, a3, a4 := r.Areas[0], r.Areas[1], r.Areas[2], r.Areas[3]
	near(t, a1.SqFt, 32.0, 1e-9)
	near(t, a1.CuFt/3.0, 26.667, 1e-3)
	near(t, a1.CuFt, 80.0, 1e-9)
	near(t, a2.SqFt, 28.274, 1e-3)
	near(t, a2.CuFt, 18.850, 1e-3)
	near(t, a3.CuFt, 20.833, 1e-3)
	near(t, a4.SqFt, 60.0, 1e-9)
	near(t, a4.CuFt, 20.0, 1e-9)
	if got := materialIDs(r); !slices.Equal(got, []Material{Topsoil, Fill}) {
		t.Errorf("materials = %v, want topsoil then fill", got)
	}
	top, fill := r.Materials[0], r.Materials[1]
	near(t, top.CuFt, 119.683, 1e-3)
	near(t, top.CuYd, 4.4327, 1e-4)
	near(t, fill.CuFt, 20.0, 1e-9)
	near(t, fill.CuYd, 0.7407, 1e-4)
	near(t, r.Totals.CuFt, 139.683, 1e-3)
	near(t, r.Totals.CuYd, 5.1734, 1e-4)
	near(t, r.Totals.OrderedCuFt, r.Totals.CuFt, 1e-12)
}

func TestEachShapeHasItsOwnAreaFormula(t *testing.T) {
	near(t, areaOf(t, "r", Rect{L: 8.0 * ft, W: 4.0 * ft})/SqFt, 32.0, 1e-9)
	near(t, areaOf(t, "c", Circle{D: 6.0 * ft})/SqFt, math.Pi*9.0, 1e-9)
	near(t, areaOf(t, "t", Triangle{B: 12.0 * ft, H: 10.0 * ft})/SqFt, 60.0, 1e-9)
	near(t, areaOf(t, "k", KnownArea{Area: 1000.0 * SqFt})/SqFt, 1000.0, 1e-9)
}

func TestBagUnitsCubicFeetDryQuartsLitres(t *testing.T) {
	r := NewRules()
	near(t, BagCuFt(1.0, CubicFeet, r.DryQuartIn3), 1.0, 0.0)
	near(t, BagCuFt(20.0, DryQuarts, r.DryQuartIn3), 0.77779, 1e-5)
	near(t, BagCuFt(40.0, Liters, r.DryQuartIn3), 1.41259, 1e-5)
	near(t, BagCuFt(1.0, Liters, r.DryQuartIn3), 1.0/28.316846592, 1e-12)
	near(t, BagCuFt(1.0, DryQuarts, r.DryQuartIn3), 67.200625/1728.0, 1e-12)
	bags := func(size float64, unit BagUnit) *float64 {
		m := mat()
		m.BagSize, m.BagUnit = size, unit
		return plan(t, withTopsoil(m)).Materials[0].Bags
	}
	some(t, bags(1.0, CubicFeet), 120.0)
	some(t, bags(0.75, CubicFeet), 160.0)
	some(t, bags(20.0, DryQuarts), 154.0)
	some(t, bags(40.0, Liters), 85.0)
	none(t, plan(t, job()).Materials[1].Bags)
}

func TestThirtyTwoDryQuartsAreOneBushel(t *testing.T) {
	q := DryQuartIn3
	near(t, BagCuFt(synDryQuarts, DryQuarts, q), 2150.42/1728.0, 1e-12)
	near(t, 9.0*BagCuFt(20.0, DryQuarts, q), 180.0*67.200625/1728.0, 1e-12)
}

func TestWeightsLooseShortTonsBankAndSettled(t *testing.T) {
	r := plan(t, job())
	top, fill := r.Materials[0], r.Materials[1]
	near(t, top.LooseLb, top.OrderedCuYd*2000.0, 1e-9)
	near(t, top.LooseLb, 8865.4, 0.05)
	near(t, top.BulkTons, 4.433, 1e-3)
	near(t, fill.LooseLb, 1851.9, 0.05)
	near(t, fill.BulkTons, 0.926, 1e-3)
	near(t, top.BankLb, top.OrderedCuYd*2500.0, 1e-9)
	near(t, top.SettledCuFt, 119.683*0.75, 1e-3)
	near(t, fill.SettledCuFt, 20.0*0.85, 1e-9)
	near(t, r.Totals.LooseLb, top.LooseLb+fill.LooseLb, 1e-9)
}

func TestWheelbarrowLoadsAndPoundsPerLoad(t *testing.T) {
	r := plan(t, job())
	top, fill := r.Materials[0], r.Materials[1]
	some(t, top.WheelbarrowLoads, 20.0)
	near(t, val(t, top.LbPerLoad), 6.0/27.0*2000.0, 1e-9)
	some(t, fill.WheelbarrowLoads, 4.0)
	near(t, val(t, fill.LbPerLoad), 6.0/27.0*2500.0, 1e-9)
	c := job()
	c.Wheelbarrow = 0.0
	without := plan(t, c).Materials[0]
	none(t, without.WheelbarrowLoads)
	none(t, without.LbPerLoad)
}

func TestExtraPercentScalesTheOrderBagsAndBulk(t *testing.T) {
	c := job()
	c.Extra = 10.0
	top := plan(t, c).Materials[0]
	near(t, top.OrderedCuFt, 119.683*1.1, 1e-3)
	near(t, top.OrderedCuYd, top.OrderedCuFt/27.0, 1e-12)
	some(t, top.Bags, math.Ceil(119.683*1.1))
	near(t, top.BulkCuYd, top.OrderedCuYd, 1e-12)
}

func TestBulkRoundsUpToTheSaleIncrementAndReportsTheSurplus(t *testing.T) {
	c := job()
	c.Increment = 0.5
	r := plan(t, c)
	top, fill := r.Materials[0], r.Materials[1]
	near(t, top.BulkCuYd, 4.5, 1e-9)
	near(t, top.BulkSurplusCuFt, (4.5-top.OrderedCuYd)*27.0, 1e-9)
	near(t, fill.BulkCuYd, 1.0, 1e-9)
	near(t, top.LooseLb, 4.5*2000.0, 1e-6)
	if len(r.Warnings) != 2 || r.Warnings[0].Material != Topsoil || r.Warnings[1].Material != Fill {
		t.Fatalf("warnings = %v, want topsoil then fill", r.Warnings)
	}
	near(t, r.Warnings[0].SurplusCuFt, top.BulkSurplusCuFt, 1e-12)
	m := mat()
	exact := plan(t, Input{
		Areas:     []Area{area("a1", Rect{L: 9.0 * ft, W: 3.0 * ft}, 12.0*in, 1.0, Topsoil)},
		Topsoil:   &m,
		Increment: 0.5,
	})
	near(t, exact.Materials[0].BulkCuYd, 1.0, 1e-9)
	if len(exact.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", exact.Warnings)
	}
	if got := plan(t, job()).Warnings; len(got) != 0 {
		t.Errorf("warnings = %v, want none", got)
	}
}

func TestCostBagsAgainstBulkByTheYard(t *testing.T) {
	m := mat()
	m.BagPrice, m.BulkPrice = 10.0, 40.0
	c := withTopsoil(m)
	c.Delivery = 60.0
	top := plan(t, c).Materials[0]
	some(t, top.Bags, 120.0)
	near(t, val(t, top.BagCost), 1200.0, 1e-9)
	near(t, val(t, top.BulkCost), 237.31, 5e-3)
	if top.Cheaper != Bulk {
		t.Errorf("cheaper = %q, want bulk", top.Cheaper)
	}
	near(t, val(t, top.BreakEvenCuFt), 7.043, 1e-3)
	near(t, val(t, top.BreakEvenCuFt), 60.0/(10.0-40.0/27.0), 1e-9)
}

func TestCostBulkByTheTonUsesTheLooseWeight(t *testing.T) {
	m := mat()
	m.BagPrice, m.BulkPrice = 10.0, 50.0
	c := withTopsoil(m)
	c.Delivery = 60.0
	c.BulkBy = Ton
	top := plan(t, c).Materials[0]
	near(t, top.BulkTons, 4.433, 1e-3)
	near(t, val(t, top.BulkCost)-60.0, top.BulkTons*50.0, 1e-9)
	near(t, val(t, top.BulkPerCuFt), 50.0*2000.0/2000.0/27.0, 1e-12)
	near(t, val(t, top.BreakEvenCuFt), 60.0/(10.0-50.0*2000.0/2000.0/27.0), 1e-9)
}

func TestCheaperAndBreakEvenStayNullWhenAPriceIsMissing(t *testing.T) {
	only := func(edit func(m *MaterialInput)) MaterialPlan {
		m := mat()
		edit(&m)
		c := withTopsoil(m)
		c.Delivery = 60.0
		return plan(t, c).Materials[0]
	}
	noBag := only(func(m *MaterialInput) { m.BulkPrice = 40.0 })
	none(t, noBag.BagCost)
	if noBag.Cheaper != "" {
		t.Errorf("cheaper = %q, want none", noBag.Cheaper)
	}
	none(t, noBag.BreakEvenCuFt)
	near(t, val(t, noBag.BulkCost), 237.31, 5e-3)
	noBulk := only(func(m *MaterialInput) { m.BagPrice = 10.0 })
	none(t, noBulk.BulkCost)
	if noBulk.Cheaper != "" {
		t.Errorf("cheaper = %q, want none", noBulk.Cheaper)
	}
	none(t, noBulk.BreakEvenCuFt)
	noSize := only(func(m *MaterialInput) { m.BagSize, m.BagPrice, m.BulkPrice = 0.0, 10.0, 40.0 })
	none(t, noSize.Bags)
	none(t, noSize.BagCost)
	none(t, noSize.BreakEvenCuFt)
	dear := only(func(m *MaterialInput) { m.BagPrice, m.BulkPrice = 1.0, 40.0 })
	none(t, dear.BreakEvenCuFt)
	if dear.Cheaper != Bags {
		t.Errorf("cheaper = %q, want bags", dear.Cheaper)
	}
	m := mat()
	m.BagPrice, m.BulkPrice = 10.0, 40.0
	free := plan(t, withTopsoil(m)).Materials[0]
	near(t, val(t, free.BreakEvenCuFt), 0.0, 1e-12)
}

func TestTripsByWeightByVolumeAndWhichOneBinds(t *testing.T) {
	trips := func(payload, bedVolume float64) MaterialPlan {
		c := job()
		c.Payload, c.BedVolume = payload, bedVolume
		return plan(t, c).Materials[0]
	}
	both := trips(1500.0, 60.0)
	some(t, both.TripsByWeight, 6.0)
	some(t, both.TripsByVolume, 2.0)
	some(t, both.Trips, 6.0)
	if both.Binding != ByWeight {
		t.Errorf("binding = %q, want weight", both.Binding)
	}
	vol := trips(20000.0, 10.0)
	if vol.Binding != ByVolume {
		t.Errorf("binding = %q, want volume", vol.Binding)
	}
	some(t, vol.Trips, 12.0)
	tie := trips(9000.0, 120.0)
	some(t, tie.TripsByWeight, 1.0)
	some(t, tie.TripsByVolume, 1.0)
	if tie.Binding != ByBoth {
		t.Errorf("binding = %q, want both", tie.Binding)
	}
	w := trips(1500.0, 0.0)
	none(t, w.TripsByVolume)
	if w.Binding != ByWeight {
		t.Errorf("binding = %q, want weight", w.Binding)
	}
	some(t, w.Trips, 6.0)
	without := trips(0.0, 0.0)
	none(t, without.Trips)
	if without.Binding != "" {
		t.Errorf("binding = %q, want none", without.Binding)
	}
	none(t, plan(t, job()).Totals.Trips)
	c := job()
	c.Payload, c.BedVolume = 1500.0, 60.0
	r := plan(t, c)
	some(t, r.Materials[1].Trips, 2.0)
	some(t, r.Totals.Trips, 8.0)
	c = job()
	c.Payload, c.Increment = 1500.0, 1.0
	rounded := plan(t, c).Materials[0]
	some(t, rounded.TripsByWeight, 6.0)
	near(t, rounded.LooseLb, 5.0*2000.0, 1e-9)
}

func TestOnlyTheMaterialsInUseAreListedTopsoilFirst(t *testing.T) {
	a := areas()
	fill := mat()
	fill.Weight = "earth-dry"
	r := plan(t, Input{Areas: []Area{a[3]}, Fill: &fill})
	if got := materialIDs(r); !slices.Equal(got, []Material{Fill}) {
		t.Errorf("materials = %v, want fill", got)
	}
	m1, m2 := mat(), mat()
	both := plan(t, Input{Areas: []Area{a[3], a[0]}, Topsoil: &m1, Fill: &m2})
	if got := materialIDs(both); !slices.Equal(got, []Material{Topsoil, Fill}) {
		t.Errorf("materials = %v, want topsoil then fill", got)
	}
}

func TestCoverageTable324SqFtPerCubicYardAtOneInch(t *testing.T) {
	r := NewRules(synWeights...)
	c, err := Coverage([]float64{0.25, 1.0, 3.0, 12.0}, r, "topsoil")
	if err != nil {
		t.Fatal(err)
	}
	near(t, c[1].SqFtPerCuYd, 324.0, 1e-9)
	near(t, c[0].SqFtPerCuYd, 1296.0, 1e-9)
	near(t, c[2].SqFtPerCuYd, 108.0, 1e-9)
	near(t, c[3].SqFtPerCuFt, 1.0, 1e-12)
	near(t, c[1].SqFtPerTon, 324.0*2000.0/2000.0, 1e-9)
	near(t, c[1].SqFtPerCuFt, 12.0, 1e-12)
	dry, err := Coverage([]float64{2.0}, r, "earth-dry")
	if err != nil {
		t.Fatal(err)
	}
	near(t, dry[0].SqFtPerTon, 162.0*2000.0/2500.0, 1e-9)
	if _, err := Coverage([]float64{1.0}, NewRules(), "topsoil"); err == nil {
		t.Error("coverage without a weight entered: no error")
	}
}

func TestInvalidInputNamesItsField(t *testing.T) {
	rules := NewRules(synWeights...)
	bad := func(input Input, field string) {
		t.Helper()
		_, err := NewPlan(input, rules)
		var ie *calculiva.InputError
		if !errors.As(err, &ie) {
			t.Errorf("%s: got %v, want an input error", field, err)
			return
		}
		if ie.Field != field {
			t.Errorf("field = %q, want %q", ie.Field, field)
		}
	}
	a := areas()
	keep := func(*Area) {}
	same := func(*Input) {}
	std := func(*MaterialInput) {}
	one := func(f func(*Area), m func(*MaterialInput), o func(*Input)) Input {
		x := a[0]
		f(&x)
		top := mat()
		m(&top)
		s := Input{Areas: []Area{x}, Topsoil: &top}
		o(&s)
		return s
	}
	with := func(x Area, m func(*MaterialInput)) Input {
		details := mat()
		m(&details)
		if x.Material == Fill {
			return Input{Areas: []Area{x}, Fill: &details}
		}
		return Input{Areas: []Area{x}, Topsoil: &details}
	}
	shaped := func(x Area, shape Shape) Area {
		x.Shape = shape
		return x
	}
	top := mat()
	bad(Input{}, "areas")
	bad(Input{Areas: []Area{a[0], a[0], a[0], a[0], a[0], a[0], a[0], a[0], a[0]}, Topsoil: &top}, "areas")
	bad(one(func(x *Area) { x.Shape = Rect{L: 40.0, W: 4.0 * ft} }, std, same), "a1-l")
	bad(one(func(x *Area) { x.Shape = Rect{L: 8.0 * ft, W: 61000.0} }, std, same), "a1-w")
	bad(one(func(x *Area) { x.Shape = Rect{L: math.NaN(), W: 4.0 * ft} }, std, same), "a1-l")
	bad(with(shaped(a[1], Circle{D: 10.0}), std), "a2-d")
	bad(with(shaped(a[3], Triangle{B: 0.0, H: 10.0 * ft}), std), "a4-b")
	bad(with(shaped(a[3], Triangle{B: 12.0 * ft, H: 70000.0}), std), "a4-h")
	bad(with(shaped(a[2], KnownArea{Area: 0.05e6}), std), "a3-area")
	bad(with(shaped(a[2], KnownArea{Area: 1e10 + 1e6}), std), "a3-area")
	bad(one(func(x *Area) { x.Depth = 2.0 }, std, same), "a1-depth")
	bad(one(func(x *Area) { x.Depth = 1600.0 }, std, same), "a1-depth")
	bad(one(func(x *Area) { x.Count = 0.0 }, std, same), "a1-count")
	bad(one(func(x *Area) { x.Count = 1.5 }, std, same), "a1-count")
	bad(one(func(x *Area) { x.Count = 101.0 }, std, same), "a1-count")
	bad(Input{Areas: []Area{a[0]}}, "topsoil-bag")
	bad(one(keep, func(m *MaterialInput) { m.BagSize = 0.01 }, same), "topsoil-bag")
	bad(one(keep, func(m *MaterialInput) { m.BagSize = 11.0 }, same), "topsoil-bag")
	bad(one(keep, func(m *MaterialInput) { m.BagSize, m.BagUnit = 500.0, DryQuarts }, same), "topsoil-bag")
	bad(one(keep, func(m *MaterialInput) { m.BagSize, m.BagUnit = 301.0, Liters }, same), "topsoil-bag")
	bad(one(keep, func(m *MaterialInput) { m.BagPrice = -1.0 }, same), "topsoil-bag-price")
	bad(one(keep, func(m *MaterialInput) { m.BulkPrice = 100001.0 }, same), "topsoil-bulk-price")
	bad(one(keep, func(m *MaterialInput) { m.Weight = "gravel" }, same), "topsoil-weight")
	bad(with(a[3], func(m *MaterialInput) { m.Weight = "x" }), "fill-weight")
	if _, err := NewPlan(one(keep, std, same), NewRules()); err == nil {
		t.Error("plan without a weight entered: no error")
	}
	bad(with(a[3], func(m *MaterialInput) { m.BagPrice = math.NaN() }), "fill-bag-price")
	bad(one(keep, std, func(s *Input) { s.Delivery = 10001.0 }), "delivery")
	bad(one(keep, std, func(s *Input) { s.Increment = 0.05 }), "increment")
	bad(one(keep, std, func(s *Input) { s.Increment = 11.0 }), "increment")
	bad(one(keep, std, func(s *Input) { s.Extra = 101.0 }), "extra")
	bad(one(keep, std, func(s *Input) { s.Extra = -1.0 }), "extra")
	bad(one(keep, std, func(s *Input) { s.Payload = 50.0 }), "payload")
	bad(one(keep, std, func(s *Input) { s.Payload = 20001.0 }), "payload")
	bad(one(keep, std, func(s *Input) { s.BedVolume = 0.5 }), "bed-volume")
	bad(one(keep, std, func(s *Input) { s.BedVolume = 201.0 }), "bed-volume")
	bad(one(keep, std, func(s *Input) { s.Wheelbarrow = 21.0 }), "wheelbarrow")
	bad(one(keep, std, func(s *Input) { s.Wheelbarrow = 0.5 }), "wheelbarrow")
	if _, err := NewPlan(one(keep, func(m *MaterialInput) { m.BagSize = 0.05 }, same), rules); err != nil {
		t.Errorf("0.05 cu ft bag rejected: %v", err)
	}
	if _, err := NewPlan(one(func(x *Area) { x.Depth = 3.0 }, std, same), rules); err != nil {
		t.Errorf("3 mm depth rejected: %v", err)
	}
}
