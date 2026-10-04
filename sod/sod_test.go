package sod

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

func near(t *testing.T, a, b, tol float64) {
	t.Helper()
	if !(math.Abs(a-b) <= tol) {
		t.Errorf("%v vs %v", a, b)
	}
}

// equal checks an exact value.
func equal[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
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

// none checks that an optional value is absent.
func none[T any](t *testing.T, got *T) {
	t.Helper()
	if got != nil {
		t.Errorf("got %v, want none", *got)
	}
}

func a(id string, shape Shape) Area {
	return Area{ID: id, Shape: shape}
}

func a1() Area { return a("a1", Rect{L: 25.0 * ft, W: 18.0 * ft, Count: 1.0}) }
func a2() Area { return a("a2", Rect{L: 30.0 * ft, W: 4.0 * ft, Count: 1.0}) }
func a3() Area { return a("a3", Triangle{B: 10.0 * ft, H: 8.0 * ft, Count: 1.0}) }
func a4() Area { return a("a4", Circle{R: 5.0 * ft, Count: 1.0}) }
func a5() Area { return a("a5", KnownArea{Area: 1000.0 * SqFt}) }

func job() Input {
	c := NewInput([]Area{a1(), a2(), a3()}, NewRules())
	c.Format = "slab-16x24"
	c.PalletMode = PalletByPieces
	c.PalletPieces = 170.0
	c.Allowance = 10.0
	c.Spare = 0.0
	c.Method = MethodSolid
	return c
}

// with returns the default job after an edit.
func with(edit func(c *Input)) Input {
	c := job()
	edit(&c)
	return c
}

func one(x Area) Input {
	return with(func(c *Input) { c.Areas = []Area{x} })
}

// synRules are the sod rules with synthetic allowances (12 % and 20 %), read
// on no publisher: the user enters the allowances.
func synRules() Rules {
	r := NewRules()
	r.Allowance, r.AllowanceComplex = 12.0, 20.0
	return r
}

func plan(t *testing.T, c Input) *Plan {
	t.Helper()
	r, err := NewPlan(c, synRules())
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

func TestRulesComeFromTheDataFiles(t *testing.T) {
	r := NewRules()
	ids := []string{}
	for _, f := range r.Formats {
		ids = append(ids, f.ID)
	}
	if want := []string{"slab-16x24", "mini-roll-18x40", "roll-24x60", "roll-18x80"}; !slices.Equal(ids, want) {
		t.Errorf("formats = %v, want %v", ids, want)
	}
	if got := r.Formats[0].PalletPieces; !slices.Equal(got, []float64{150.0, 170.0, 189.0}) {
		t.Errorf("slab pallet pieces = %v", got)
	}
	if got := r.Formats[1].PalletPieces; !slices.Equal(got, []float64{80.0, 90.0, 100.0}) {
		t.Errorf("mini roll pallet pieces = %v", got)
	}
	if got := r.Formats[2].PalletPieces; got != nil {
		t.Errorf("roll pallet pieces = %v, want none", got)
	}
	equal(t, r.Allowance, 0.0)
	equal(t, r.AllowanceComplex, 0.0)
	equal(t, r.PalletLb, [2]float64{1500.0, 3000.0})
	near(t, r.TopsoilMM[0], 4.0*in, 1e-9)
	near(t, r.OrganicLayerMaxMM, 2.0*in, 1e-9)
	equal(t, r.OrganicRate, [2]float64{1.0, 2.0})
	type species struct {
		id      string
		spacing float64
		plug    float64
	}
	sp := []species{}
	for _, s := range r.Species {
		sp = append(sp, species{s.ID, math.Round(s.SpacingMM/in*1e6) / 1e6, s.PlugIn})
	}
	want := []species{
		{"st-augustinegrass", 12.0, 2.0},
		{"centipedegrass", 6.0, 2.0},
		{"zoysiagrass", 6.0, 2.0},
		{"bermudagrass", 12.0, 2.0},
	}
	if !slices.Equal(sp, want) {
		t.Errorf("species = %v, want %v", sp, want)
	}
	equal(t, r.PlugsPerSqYd2In, 324.0)
}

func TestDefaultExampleTwoRectsATriangleTotalsAndPallets(t *testing.T) {
	r := plan(t, job())
	x1, x2, x3 := r.Areas[0], r.Areas[1], r.Areas[2]
	near(t, r.PieceSqFt, 2.6667, 1e-4)
	l := x1.Layout
	if l == nil {
		t.Fatal("a1 has no layout")
	}
	equal(t, len(l.Rows), 14)
	near(t, l.Rows[len(l.Rows)-1].Width, 8.0*in, 1e-9)
	equal(t, [6]int{l.Installed, l.Cut, l.Pairs, l.Strips, l.StripBought, l.Bought}, [6]int{182, 26, 7, 12, 6, 169})
	near(t, x1.SqFt, 450.0, 1e-9)
	near(t, val(t, x1.ExtraPct), 0.148, 1e-3)
	equal(t, math.Ceil(450.0*1.1/r.PieceSqFt-1e-9), 186.0)
	l2 := x2.Layout
	if l2 == nil {
		t.Fatal("a2 has no layout")
	}
	equal(t, [4]int{len(l2.Rows), l2.Installed, l2.Cut, l2.Pairs}, [4]int{3, 46, 2, 1})
	equal(t, x2.Bought, 45.0)
	near(t, x2.SqFt, 120.0, 1e-9)
	near(t, val(t, x2.ExtraPct), 0.0, 1e-9)
	equal(t, x3.Mode, ModeArea)
	near(t, x3.SqFt, 40.0, 1e-9)
	equal(t, x3.Bought, 17.0)
	near(t, r.Totals.SqFt, 610.0, 1e-9)
	equal(t, r.Totals.Pieces, 231.0)
	equal(t, r.Totals.Flat, math.Ceil(610.0*1.12/r.PieceSqFt-1e-9))
	equal(t, r.Totals.Flat, 257.0)
	equal(t, r.Pallets, Pallets{PerPallet: 170.0, Full: 1.0, Loose: 61.0, PalletsUp: 2.0, Leftover: 109.0})
	equal(t, r.WeightLb, [2]float64{3000.0, 6000.0})
	none(t, r.Trips)
	wantKinds(t, r)
}

func TestSoilPreparationTopsoilOrganicLayerAndRate(t *testing.T) {
	p := plan(t, job()).Prep
	near(t, p.TopsoilCuFt, 203.333, 1e-3)
	near(t, p.TopsoilCuYd, 7.531, 1e-3)
	near(t, p.OrganicCuFt, 50.833, 1e-3)
	near(t, p.OrganicCuYd, 1.883, 1e-3)
	near(t, val(t, p.EquivalentRate), 1.883/0.61, 1e-3)
	none(t, p.EquivalentLayerMM)
	q := plan(t, with(func(c *Input) { c.Organic, c.OrganicRate = OrganicRate, 1.0 })).Prep
	near(t, q.OrganicCuYd, 0.61, 1e-9)
	near(t, val(t, q.EquivalentLayerMM)/in, 0.324, 1e-9)
	none(t, q.EquivalentRate)
	n := plan(t, with(func(c *Input) { c.Organic = OrganicNone })).Prep
	equal(t, n.OrganicCuFt, 0.0)
	none(t, n.EquivalentRate)
	none(t, n.EquivalentLayerMM)
}

func TestLayoutTestCases24By18FtAndACircle(t *testing.T) {
	l, err := LayRect(24.0*ft, 18.0*ft, 16.0*in, 24.0*in)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, [4]int{l.Installed, l.Pairs, l.Strips, l.Bought}, [4]int{175, 7, 11, 163})
	c := plan(t, one(a4()))
	near(t, c.Areas[0].SqFt, 78.54, 1e-2)
	equal(t, c.Areas[0].Bought, 33.0)
	two := plan(t, one(a("a1", Rect{L: 25.0 * ft, W: 18.0 * ft, Count: 2.0})))
	equal(t, two.Areas[0].Bought, 338.0)
	near(t, two.Totals.SqFt, 900.0, 1e-9)
	shared := 0
	for _, p := range l.Pieces {
		if p.Shared {
			shared++
		}
	}
	if !(shared >= 7*2+11) {
		t.Errorf("shared pieces = %d, want at least %d", shared, 7*2+11)
	}
}

func TestSpareOnTheLayoutRowsOnly(t *testing.T) {
	r := plan(t, with(func(c *Input) { c.Spare = 5.0 }))
	equal(t, r.Totals.Spare, math.Ceil((169.0+45.0)*0.05))
	equal(t, r.Totals.Pieces, 231.0+11.0)
	spared := one(a3())
	spared.Spare = 30.0
	equal(t, plan(t, spared).Totals.Pieces, 17.0)
	equal(t, plan(t, with(func(c *Input) { c.Allowance = 0.0 })).Areas[2].Bought, 15.0)
}

func TestMoreThan2500PiecesAreaCountAndANoLayoutWarning(t *testing.T) {
	r := plan(t, one(a("a1", Rect{L: 120.0 * ft, W: 80.0 * ft, Count: 1.0})))
	equal(t, r.Areas[0].Mode, ModeNoLayout)
	none(t, r.Areas[0].Layout)
	equal(t, r.Areas[0].Bought, math.Ceil(9600.0*1.1/(8.0/3.0)-1e-9))
	if !slices.Equal(r.Warnings, []Warning{{Kind: NoLayout, Area: "a1"}}) {
		t.Errorf("warnings = %v, want one no-layout on a1", r.Warnings)
	}
	strip := one(a("a1", Rect{L: 300.0 * ft, W: 1.0 * ft, Count: 1.0}))
	strip.Format = "roll-18x80"
	equal(t, plan(t, strip).Areas[0].Mode, ModeLayout)
}

func TestFormatsCustomPieceAndPalletInSquareFeet(t *testing.T) {
	mini := plan(t, with(func(c *Input) { c.Format, c.PalletPieces = "mini-roll-18x40", 90.0 }))
	near(t, mini.PieceSqFt, 5.0, 1e-9)
	sq := plan(t, with(func(c *Input) { c.PalletMode, c.PalletSqFt = PalletBySqFt, 450.0 }))
	equal(t, sq.PerPallet, 168.0)
	cu := plan(t, with(func(c *Input) { c.Format, c.PieceW, c.PieceL = "custom", 16.0*in, 24.0*in }))
	equal(t, cu.Totals.Pieces, 231.0)
	rules := NewRules()
	if rules.Formats[0].SoldAsSqFt == nil {
		t.Fatal("the slab format has no sold-as pallets")
	}
	table := FormatTable(rules, rules.Formats[0].SoldAsSqFt, 610.0)
	if got := table[0].PerPallet; !slices.Equal(got, []float64{150.0, 169.0, 188.0}) {
		t.Errorf("slab per pallet = %v", got)
	}
	equal(t, table[0].ForLawn, 229.0)
	if got := table[1].PerPallet; !slices.Equal(got, []float64{80.0, 90.0, 100.0}) {
		t.Errorf("mini roll per pallet = %v", got)
	}
	equal(t, table[2].ForLawn, 61.0)
}

func TestPlugsStAugustineZoysiaAnd3And4InPlugs(t *testing.T) {
	plugs := func(species string, spacing, plugSize float64) Input {
		c := one(a5())
		c.Method, c.Species, c.Spacing, c.PlugSize = MethodPlugs, species, spacing, plugSize
		return c
	}
	p := func(species string, spacing, plugSize float64) *PlugPlan {
		t.Helper()
		plug := plan(t, plugs(species, spacing, plugSize)).Plug
		if plug == nil {
			t.Fatal("no plug plan")
		}
		return plug
	}
	st := p("st-augustinegrass", 12.0*in, 2.0)
	equal(t, st.Count, 1000.0)
	near(t, st.SodSqFt, 27.778, 1e-3)
	near(t, st.SodSqYd, 3.086, 1e-3)
	equal(t, st.Pieces, 11.0)
	near(t, st.PerSqYd, 324.0, 1e-9)
	if st.TableSqFt == nil || st.TablePieces == nil {
		t.Fatal("St. Augustine has no table range")
	}
	near(t, st.TableSqFt[0], 30.0, 1e-9)
	near(t, st.TableSqFt[1], 50.0, 1e-9)
	equal(t, *st.TablePieces, [2]float64{12.0, 19.0})
	z := p("zoysiagrass", 6.0*in, 2.0)
	equal(t, z.Count, 4000.0)
	near(t, z.SodSqFt, 111.111, 1e-3)
	equal(t, z.Pieces, 42.0)
	if z.TablePieces == nil {
		t.Fatal("zoysia has no table range")
	}
	equal(t, *z.TablePieces, [2]float64{38.0, 57.0})
	three := p("st-augustinegrass", 12.0*in, 3.0)
	near(t, three.PerSqYd, 144.0, 1e-9)
	none(t, three.TableSqFt)
	near(t, p("custom", 12.0*in, 4.0).PerSqYd, 81.0, 1e-9)
	r := plan(t, plugs("st-augustinegrass", 12.0*in, 2.0))
	equal(t, r.Totals.Pieces, 11.0)
	none(t, r.Areas[0].Layout)
	wantKinds(t, plan(t, plugs("zoysiagrass", 12.0*in, 2.0)), Spacing)
	wantKinds(t, plan(t, plugs("custom", 9.0*in, 2.0)))
}

func TestPreparationWarnings(t *testing.T) {
	k := func(edit func(c *Input)) *Plan { return plan(t, with(edit)) }
	wantKinds(t, k(func(c *Input) { c.Topsoil = 3.0 * in }), TopsoilThin)
	wantKinds(t, k(func(c *Input) { c.Topsoil = 0.0 }))
	wantKinds(t, k(func(c *Input) { c.OrganicIn = 3.0 * in }), OrganicThick)
	wantKinds(t, k(func(c *Input) { c.Organic, c.OrganicRate = OrganicRate, 3.0 }), RateOutside)
	wantKinds(t, k(func(c *Input) { c.Organic, c.OrganicRate = OrganicRate, 1.5 }))
}

func TestCostByPalletPieceAndSquareFootWeightAndTrips(t *testing.T) {
	pal := plan(t, with(func(c *Input) { c.PriceMode, c.Price, c.Delivery = PricePerPallet, 300.0, 75.0 })).Cost
	near(t, val(t, pal.Whole), 675.0, 1e-9)
	none(t, pal.Mixed)
	pc := plan(t, with(func(c *Input) { c.PriceMode, c.Price = PricePerPiece, 1.0 })).Cost
	near(t, val(t, pc.Whole), 340.0, 1e-9)
	near(t, val(t, pc.Mixed), 231.0, 1e-9)
	sf := plan(t, with(func(c *Input) { c.PriceMode, c.Price, c.Delivery = PricePerSqFt, 0.5, 10.0 })).Cost
	near(t, val(t, sf.Whole), 340.0*4.0/3.0+10.0, 1e-9)
	near(t, val(t, sf.Mixed), 231.0*4.0/3.0+10.0, 1e-9)
	none(t, plan(t, with(func(c *Input) { c.Price = 0.0 })).Cost.Whole)
	trips := plan(t, with(func(c *Input) { c.Payload = 2000.0 })).Trips
	if trips == nil {
		t.Fatal("no trips")
	}
	equal(t, *trips, [2]float64{2.0, 3.0})
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
	rect := func(l, w, count float64) Input { return one(a("a1", Rect{L: l, W: w, Count: count})) }
	bad(with(func(c *Input) { c.Areas = []Area{} }), "areas")
	bad(with(func(c *Input) { c.Areas = []Area{a1(), a1(), a1(), a1(), a1(), a1(), a1(), a1(), a1()} }), "areas")
	bad(rect(200.0, 18.0*ft, 1.0), "a1-l")
	bad(rect(25.0*ft, 301.0*ft, 1.0), "a1-w")
	bad(rect(25.0*ft, 18.0*ft, 0.0), "a1-count")
	bad(rect(25.0*ft, 18.0*ft, 21.0), "a1-count")
	bad(rect(25.0*ft, 18.0*ft, 1.5), "a1-count")
	bad(one(a("a4", Circle{R: 151.0 * ft, Count: 1.0})), "a4-r")
	bad(one(a("a3", Triangle{B: math.NaN(), H: 8.0 * ft, Count: 1.0})), "a3-b")
	bad(one(a("a5", KnownArea{Area: 0.05e6})), "a5-area")
	bad(with(func(c *Input) { c.Format = "sheet" }), "format")
	bad(with(func(c *Input) { c.Format, c.PieceW, c.PieceL = "custom", 3.0*in, 24.0*in }), "piece-w")
	bad(with(func(c *Input) { c.Format, c.PieceW, c.PieceL = "custom", 16.0*in, 121.0*in }), "piece-l")
	bad(with(func(c *Input) { c.PalletPieces = 0.0 }), "pallet-pieces")
	bad(with(func(c *Input) { c.PalletPieces = 2001.0 }), "pallet-pieces")
	bad(with(func(c *Input) { c.PalletMode, c.PalletSqFt = PalletBySqFt, 49.0 }), "pallet-sqft")
	bad(with(func(c *Input) { c.Allowance = 31.0 }), "allowance")
	bad(with(func(c *Input) { c.Spare = -1.0 }), "spare")
	bad(with(func(c *Input) { c.Method, c.Species = MethodPlugs, "fescue" }), "species")
	bad(with(func(c *Input) { c.Method, c.Spacing = MethodPlugs, 3.0*in }), "spacing")
	bad(with(func(c *Input) { c.Method, c.PlugSize = MethodPlugs, 5.0 }), "plug-size")
	bad(with(func(c *Input) { c.Topsoil = 13.0 * in }), "topsoil")
	bad(with(func(c *Input) { c.OrganicIn = 5.0 * in }), "organic-in")
	bad(with(func(c *Input) { c.Organic, c.OrganicRate = OrganicRate, 5.0 }), "organic-rate")
	bad(with(func(c *Input) { c.Price = 10001.0 }), "price")
	bad(with(func(c *Input) { c.Delivery = -1.0 }), "delivery")
	bad(with(func(c *Input) { c.Payload = 50.0 }), "payload")
}
