package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/asphalt"
	"github.com/calculiva/calculiva-go/mulch"
	"github.com/calculiva/calculiva-go/sod"
	"github.com/calculiva/calculiva-go/soil"
)

// cli runs the command and decodes its JSON output into v.
func cli(t *testing.T, v any, args ...string) {
	t.Helper()
	var out, errb bytes.Buffer
	if code := run(append(args, "--json"), &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if err := json.Unmarshal(out.Bytes(), v); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
}

func eq(t *testing.T, name string, got, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func eqPtr(t *testing.T, name string, got, want *float64) {
	t.Helper()
	if (got == nil) != (want == nil) || (got != nil && *got != *want) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

// Synthetic test values, labelled as such: none is a publisher's figure.
func TestMulchMatchesAPI(t *testing.T) {
	var got mulchJSON
	cli(t, &got, "mulch", "--length", "40", "--width", "3", "--count", "2", "--depth", "3", "--existing", "1", "--bag", "2", "--extra", "10")
	plan, err := mulch.NewPlan(mulch.Input{
		Areas:   []mulch.Area{{ID: "bed", Kind: mulch.Bed{L: 40 * calculiva.Foot, W: 3 * calculiva.Foot, Count: 2}, Target: 3 * calculiva.Inch, Existing: 1 * calculiva.Inch}},
		BagSize: 2, Extra: 10,
	}, mulch.NewRules())
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "cu_ft", got.CuFt, plan.Totals.CuFt)
	eq(t, "ordered_cu_ft", got.OrderedCuFt, plan.Totals.OrderedCuFt)
	eq(t, "bulk_cu_yd", got.BulkCuYd, plan.Buy.BulkCuYd)
	eqPtr(t, "bags", got.Bags, plan.Buy.Bags)
	eq(t, "sq_ft", got.SqFt, 240)
}

func TestSoilMatchesAPI(t *testing.T) {
	var got soilJSON
	cli(t, &got, "soil", "--length", "8", "--width", "4", "--count", "3", "--depth", "10", "--loose", "2000", "--bank", "2500", "--load-factor", "0.75", "--bag", "1", "--wheelbarrow", "5")
	w := soil.Weight{ID: "w", Loose: 2000, Bank: 2500, LoadFactor: 0.75}
	plan, err := soil.NewPlan(soil.Input{
		Areas:       []soil.Area{{ID: "beds", Shape: soil.Rect{L: 8 * calculiva.Foot, W: 4 * calculiva.Foot}, Depth: 10 * calculiva.Inch, Count: 3, Material: soil.Topsoil}},
		Topsoil:     &soil.MaterialInput{BagSize: 1, BagUnit: soil.CubicFeet, Weight: "w"},
		Wheelbarrow: 5,
	}, soil.NewRules(w))
	if err != nil {
		t.Fatal(err)
	}
	m := plan.Materials[0]
	eq(t, "cu_ft", got.CuFt, m.CuFt)
	eq(t, "loose_lb", got.LooseLb, m.LooseLb)
	eq(t, "bulk_tons", got.BulkTons, m.BulkTons)
	eqPtr(t, "bags", got.Bags, m.Bags)
	eqPtr(t, "bank_lb", got.BankLb, &m.BankLb)
	eqPtr(t, "settled_cu_ft", got.SettledCuFt, &m.SettledCuFt)
	eqPtr(t, "wheelbarrow_loads", got.WheelbarrowLoads, m.WheelbarrowLoads)
}

func TestSodMatchesAPI(t *testing.T) {
	var got sodJSON
	cli(t, &got, "sod", "--length", "25", "--width", "18", "--format", "slab-16x24", "--pallet-pieces", "170", "--price", "200")
	rules := sod.NewRules()
	in := sod.NewInput([]sod.Area{{ID: "lawn", Shape: sod.Rect{L: 25 * calculiva.Foot, W: 18 * calculiva.Foot, Count: 1}}}, rules)
	in.Format, in.PalletPieces, in.Price = "slab-16x24", 170, 200
	plan, err := sod.NewPlan(in, rules)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "sq_ft", got.SqFt, plan.Totals.SqFt)
	eq(t, "pieces", got.Pieces, plan.Totals.Pieces)
	eq(t, "pallets", got.Pallets, plan.Pallets.PalletsUp)
	eq(t, "leftover", got.Leftover, plan.Pallets.Leftover)
	eqPtr(t, "cost_whole", got.CostWhole, plan.Cost.Whole)
}

func TestAsphaltMatchesAPI(t *testing.T) {
	var got asphaltJSON
	cli(t, &got, "asphalt", "--width", "10", "--length", "25", "--thickness", "4", "--pcf", "150", "--max-lift", "2.5", "--load", "3")
	in := asphalt.NewInput([]asphalt.Zone{{ID: "d", Shape: asphalt.Rect{W: 10 * calculiva.Foot, L: 25 * calculiva.Foot}}},
		[]asphalt.Course{{ID: "c", Thickness: 4 * calculiva.Inch}}, asphalt.PCF(150))
	in.Load = 3
	rules := asphalt.NewRules()
	rules.MaxLiftStaticIn = 2.5
	plan, err := asphalt.NewPlan(in, rules)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "tons", got.Tons, plan.Tons)
	eq(t, "cu_ft", got.CuFt, plan.CuFt)
	eq(t, "loads", got.Loads, plan.Loads)
	eq(t, "lifts", got.Lifts, plan.Courses[0].Lifts)
}

func TestAsphaltMixDesignMatchesAPI(t *testing.T) {
	var got asphaltJSON
	cli(t, &got, "asphalt", "--shape", "round", "--radius", "6", "--part", "0.5", "--thickness", "2",
		"--rice", "2.5", "--compaction", "93", "--water-pcf", "62", "--max-lift", "3", "--roller", "vibratory")
	in := asphalt.NewInput([]asphalt.Zone{{ID: "r", Shape: asphalt.Round{R: 6 * calculiva.Foot, Part: 0.5}}},
		[]asphalt.Course{{ID: "c", Thickness: 2 * calculiva.Inch}}, asphalt.MixDesign{Rice: 2.5, Compaction: 93})
	in.Roller = asphalt.Vibratory
	rules := asphalt.NewRules()
	rules.WaterPCF, rules.MaxLiftVibratoryIn = 62, 3
	plan, err := asphalt.NewPlan(in, rules)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "pcf", got.PCF, plan.PCF)
	eq(t, "tons", got.Tons, plan.Tons)
}

func TestMissingRequiredFlags(t *testing.T) {
	cases := []struct {
		args []string
		flag string
	}{
		{[]string{"mulch", "--length", "40", "--width", "3"}, "--depth"},
		{[]string{"mulch", "--shape", "ring", "--depth", "3"}, "--radius"},
		{[]string{"soil", "--length", "8", "--width", "4", "--depth", "10"}, "--loose"},
		{[]string{"soil", "--length", "8", "--width", "4", "--loose", "2000"}, "--depth"},
		{[]string{"sod", "--length", "25", "--width", "18"}, "--pallet-pieces"},
		{[]string{"sod", "--length", "25", "--width", "18", "--format", "custom", "--pallet-pieces", "170"}, "--piece-width"},
		{[]string{"asphalt", "--width", "10", "--length", "25", "--thickness", "4", "--max-lift", "2.5"}, "--pcf"},
		{[]string{"asphalt", "--width", "10", "--length", "25", "--thickness", "4", "--pcf", "150"}, "--max-lift"},
		{[]string{"asphalt", "--width", "10", "--length", "25", "--thickness", "4", "--rice", "2.5", "--max-lift", "2"}, "--compaction"},
		{[]string{"asphalt", "--width", "10", "--length", "25", "--thickness", "4", "--pcf", "150", "--max-lift", "2", "--nmas", "12.5"}, "--min-lift-multiple"},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		code := run(c.args, &out, &errb)
		if code != 2 || !strings.Contains(errb.String(), "missing required flag "+c.flag) || out.Len() != 0 {
			t.Errorf("%v: exit %d, stderr %q, stdout %q; want exit 2 naming %s", c.args, code, errb.String(), out.String(), c.flag)
		}
	}
}

func TestModuleErrorExitsOne(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"asphalt", "--width", "10", "--length", "25", "--thickness", "4", "--pcf", "500", "--max-lift", "2"}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "unit weight") {
		t.Errorf("exit %d, stderr %q", code, errb.String())
	}
}

func TestVersionHelpUnknown(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"version"}, &out, &errb); code != 0 || out.String() != "calculiva "+version+"\n" {
		t.Errorf("version: exit %d, %q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"help"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "asphalt") {
		t.Errorf("help: exit %d", code)
	}
	out.Reset()
	if code := run([]string{"sod", "-h"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "-pallet-pieces") {
		t.Errorf("sod -h: exit %d, %q", code, out.String())
	}
	errb.Reset()
	if code := run([]string{"gravel"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("unknown: exit %d", code)
	}
}

func TestTextOutput(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mulch", "--length", "40", "--width", "3", "--count", "2", "--depth", "3", "--existing", "1", "--bag", "2"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	for _, s := range []string{"40.0 cu ft", "20 x 2 cu ft", "https://calculiva.com/landscaping/mulch-calculator/"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("missing %q in\n%s", s, out.String())
		}
	}
}
