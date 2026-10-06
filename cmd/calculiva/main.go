// Command calculiva is an offline command-line front end to the Calculiva
// landscaping estimators: mulch, topsoil and fill, sod and hot mix asphalt.
//
// It calls the public packages of github.com/calculiva/calculiva-go and adds
// no constant of its own. Where a package needs a value that only the user can
// supply (the loose weight of a soil, the size of a sod pallet, the unit weight
// of an asphalt mix, the maximum lift of a roller), the flag is required and the
// command stops with a message when it is missing: no default is invented.
//
// Lengths are entered in feet, depths and thicknesses in inches, areas in
// square feet. Add --json for machine-readable output.
//
//	calculiva mulch --length 40 --width 3 --count 2 --depth 3 --existing 1 --bag 2
//	calculiva soil --length 8 --width 4 --count 3 --depth 10 --loose 2000
//	calculiva sod --length 25 --width 18 --pallet-pieces 170
//	calculiva asphalt --width 10 --length 25 --thickness 4 --pcf 150 --max-lift 2.5
//
// The same calculators run online, with their sources and method:
//   - https://calculiva.com/landscaping/
//   - https://calculiva.com/landscaping/mulch-calculator/
//   - https://calculiva.com/landscaping/soil-calculator/
//   - https://calculiva.com/landscaping/sod-calculator/
//   - https://calculiva.com/landscaping/asphalt-calculator/
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/asphalt"
	"github.com/calculiva/calculiva-go/mulch"
	"github.com/calculiva/calculiva-go/sod"
	"github.com/calculiva/calculiva-go/soil"
)

// version is the version printed by "calculiva version".
var version = "0.2.0"

const landscaping = "https://calculiva.com/landscaping/"

const (
	ft   = calculiva.Foot
	inch = calculiva.Inch
	sqft = calculiva.Foot * calculiva.Foot
)

func page(name string) string { return landscaping + name + "-calculator/" }

// usageError is a missing or invalid flag (exit status 2).
type usageError string

func (e usageError) Error() string { return string(e) }

// helpText is the help of a command, asked with -h.
type helpText string

func (h helpText) Error() string { return string(h) }

// result is either a JSON value or a text.
type result struct {
	data any
	text string
}

type command struct {
	name, summary string
	run           func(args []string) (result, error)
}

var commands = []command{
	{"mulch", "mulch for beds, tree rings, known areas, play areas and swings", runMulch},
	{"soil", "topsoil or fill dirt: volume, bags, bulk, weight and trips", runSoil},
	{"sod", "sod pieces laid out, pallets and cost", runSod},
	{"asphalt", "hot mix asphalt: tons, lifts and truck loads", runAsphalt},
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
		usage(stdout)
		return 0
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, "calculiva", version)
		return 0
	}
	for _, c := range commands {
		if c.name != args[0] {
			continue
		}
		res, err := c.run(args[1:])
		var h helpText
		var u usageError
		switch {
		case errors.As(err, &h):
			fmt.Fprint(stdout, string(h))
			return 0
		case errors.As(err, &u):
			fmt.Fprintf(stderr, "calculiva %s: %s\nRun 'calculiva %s -h' for the flags.\n", c.name, u, c.name)
			return 2
		case err != nil:
			fmt.Fprintf(stderr, "calculiva %s: %s\n", c.name, err)
			return 1
		}
		if res.data != nil {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(res.data); err != nil {
				fmt.Fprintf(stderr, "calculiva %s: %s\n", c.name, err)
				return 1
			}
			return 0
		}
		fmt.Fprint(stdout, res.text)
		return 0
	}
	fmt.Fprintf(stderr, "calculiva: unknown command %q\n\n", args[0])
	usage(stderr)
	return 2
}

func usage(w io.Writer) {
	fmt.Fprint(w, "calculiva: offline landscaping estimators from https://calculiva.com/landscaping/\n\n")
	fmt.Fprint(w, "Usage: calculiva <command> [flags]\n\nCommands:\n")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-8s %s\n", c.name, c.summary)
	}
	fmt.Fprint(w, "  version  print the version\n  help     print this help\n\n")
	fmt.Fprint(w, "Lengths in feet, depths in inches, areas in square feet. Add --json for JSON.\n")
	fmt.Fprint(w, "Run 'calculiva <command> -h' for the flags of a command.\n")
}

func newFlags(name, about string) (*flag.FlagSet, *bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: calculiva %s [flags]\n\n%s\nOnline: %s\n\nFlags:\n", name, about, page(name))
		fs.PrintDefaults()
	}
	return fs, fs.Bool("json", false, "print the result as JSON")
}

func parse(fs *flag.FlagSet, args []string) error {
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		var b strings.Builder
		fs.SetOutput(&b)
		fs.Usage()
		fs.SetOutput(io.Discard)
		return helpText(b.String())
	}
	if err != nil {
		return usageError(err.Error())
	}
	if fs.NArg() > 0 {
		return usageError(fmt.Sprintf("unexpected argument %q", fs.Arg(0)))
	}
	return nil
}

func isSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) { found = found || f.Name == name })
	return found
}

// need stops at the first required flag left out; each pair is a flag name and
// what to enter.
func need(fs *flag.FlagSet, pairs ...[2]string) error {
	for _, p := range pairs {
		if !isSet(fs, p[0]) {
			return usageError(fmt.Sprintf("missing required flag --%s: %s", p[0], p[1]))
		}
	}
	return nil
}

func choice(name, v string, opts ...string) error {
	for _, o := range opts {
		if v == o {
			return nil
		}
	}
	return usageError(fmt.Sprintf("--%s must be one of %s, not %q", name, strings.Join(opts, ", "), v))
}

func done(asJSON bool, data any, text string) (result, error) {
	if asJSON {
		return result{data: data}, nil
	}
	return result{text: text}, nil
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func money(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f USD", *v)
}

// ---- mulch ----

type mulchJSON struct {
	Calculator       string   `json:"calculator"`
	Page             string   `json:"page"`
	Shape            string   `json:"shape"`
	SqFt             float64  `json:"sq_ft"`
	AddedDepthIn     float64  `json:"added_depth_in"`
	CuFt             float64  `json:"cu_ft"`
	CuYd             float64  `json:"cu_yd"`
	OrderedCuFt      float64  `json:"ordered_cu_ft"`
	OrderedCuYd      float64  `json:"ordered_cu_yd"`
	BulkCuYd         float64  `json:"bulk_cu_yd"`
	Bags             *float64 `json:"bags,omitempty"`
	BagCost          *float64 `json:"bag_cost,omitempty"`
	BulkCost         *float64 `json:"bulk_cost,omitempty"`
	Cheaper          string   `json:"cheaper,omitempty"`
	WheelbarrowLoads *float64 `json:"wheelbarrow_loads,omitempty"`
	Warnings         []string `json:"warnings"`
}

func runMulch(args []string) (result, error) {
	fs, asJSON := newFlags("mulch", "Mulch volume, bags and bulk for a bed, a tree ring, a known area,\na play structure or a swing (CPSC depth and extent for the last two).")
	shape := fs.String("shape", "bed", "bed, ring, area, play or swing")
	length := fs.Float64("length", 0, "length, ft (bed, play)")
	width := fs.Float64("width", 0, "width, ft (bed, play)")
	radius := fs.Float64("radius", 0, "outer ring radius, ft (ring)")
	trunk := fs.Float64("trunk", 0, "trunk diameter, in (ring)")
	gap := fs.Float64("gap", 0, "bare gap at the trunk, in (ring)")
	area := fs.Float64("area", 0, "known area, sq ft (area)")
	height := fs.Float64("height", 0, "equipment height or swing top bar height, ft (play, swing)")
	beam := fs.Float64("beam", 0, "swing top bar length, ft (swing)")
	count := fs.Float64("count", 1, "number of identical beds or rings")
	depth := fs.Float64("depth", 0, "target depth, in (required for bed, ring and area)")
	existing := fs.Float64("existing", 0, "depth already in place, in")
	texture := fs.String("texture", "coarse", "coarse or fine (shredded)")
	bag := fs.Float64("bag", 0, "bag size, cu ft (0 for no bags)")
	bagPrice := fs.Float64("bag-price", 0, "price of one bag, USD")
	bulkPrice := fs.Float64("bulk-price", 0, "bulk price per cu yd, USD")
	delivery := fs.Float64("delivery", 0, "delivery charge, USD")
	extra := fs.Float64("extra", 0, "extra, percent")
	wheelbarrow := fs.Float64("wheelbarrow", 0, "wheelbarrow volume, cu ft (0 for none)")
	if err := parse(fs, args); err != nil {
		return result{}, err
	}
	var kind mulch.Kind
	var req [][2]string
	switch *shape {
	case "bed":
		req = [][2]string{{"length", "the bed length in feet"}, {"width", "the bed width in feet"}}
		kind = mulch.Bed{L: *length * ft, W: *width * ft, Count: *count}
	case "ring":
		req = [][2]string{{"radius", "the outer ring radius in feet"}}
		kind = mulch.Ring{R: *radius * ft, Trunk: *trunk * inch, Gap: *gap * inch, Count: *count}
	case "area":
		req = [][2]string{{"area", "the area in square feet"}}
		kind = mulch.KnownArea{Area: *area * sqft}
	case "play":
		req = [][2]string{{"length", "the structure length in feet"}, {"width", "the structure width in feet"}, {"height", "the equipment height in feet"}}
		kind = mulch.Play{L: *length * ft, W: *width * ft, Height: *height * ft}
	case "swing":
		req = [][2]string{{"beam", "the top bar length in feet"}, {"height", "the top bar height in feet"}}
		kind = mulch.Swing{Beam: *beam * ft, Height: *height * ft}
	default:
		return result{}, choice("shape", *shape, "bed", "ring", "area", "play", "swing")
	}
	if *shape == "bed" || *shape == "ring" || *shape == "area" {
		req = append(req, [2]string{"depth", "the target mulch depth in inches"})
	}
	if err := need(fs, req...); err != nil {
		return result{}, err
	}
	if err := choice("texture", *texture, "coarse", "fine"); err != nil {
		return result{}, err
	}
	tex := mulch.Coarse
	if *texture == "fine" {
		tex = mulch.Fine
	}
	in := mulch.Input{
		Areas:       []mulch.Area{{ID: *shape, Kind: kind, Target: *depth * inch, Existing: *existing * inch}},
		Texture:     tex,
		BagSize:     *bag,
		BagPrice:    *bagPrice,
		BulkPrice:   *bulkPrice,
		Delivery:    *delivery,
		Extra:       *extra,
		Wheelbarrow: *wheelbarrow,
	}
	plan, err := mulch.NewPlan(in, mulch.NewRules())
	if err != nil {
		return result{}, err
	}
	a := plan.Areas[0]
	out := mulchJSON{
		Calculator: "mulch", Page: page("mulch"), Shape: *shape,
		SqFt: a.SqFt * a.Count, AddedDepthIn: a.AddedDepth / inch,
		CuFt: plan.Totals.CuFt, CuYd: plan.Totals.CuYd,
		OrderedCuFt: plan.Totals.OrderedCuFt, OrderedCuYd: plan.Totals.OrderedCuYd,
		BulkCuYd: plan.Buy.BulkCuYd, Bags: plan.Buy.Bags, BagCost: plan.Buy.BagCost,
		BulkCost: plan.Buy.BulkCost, Cheaper: string(plan.Buy.Cheaper),
		WheelbarrowLoads: plan.WheelbarrowLoads, Warnings: []string{},
	}
	for _, w := range plan.Warnings {
		out.Warnings = append(out.Warnings, string(w.Kind))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Mulch, %s: %.1f sq ft, %.2f in added\n", *shape, out.SqFt, out.AddedDepthIn)
	fmt.Fprintf(&b, "Volume:  %.1f cu ft (%.2f cu yd)\n", out.CuFt, out.CuYd)
	fmt.Fprintf(&b, "Ordered: %.1f cu ft (%.2f cu yd) with %s %% extra\n", out.OrderedCuFt, out.OrderedCuYd, num(*extra))
	if out.Bags != nil {
		fmt.Fprintf(&b, "Bags:    %.0f x %s cu ft, %s\n", *out.Bags, num(*bag), money(out.BagCost))
	}
	fmt.Fprintf(&b, "Bulk:    %.2f cu yd, %s\n", out.BulkCuYd, money(out.BulkCost))
	if out.Cheaper != "" {
		fmt.Fprintf(&b, "Cheaper: %s\n", out.Cheaper)
	}
	if out.WheelbarrowLoads != nil {
		fmt.Fprintf(&b, "Wheelbarrow loads: %.0f\n", *out.WheelbarrowLoads)
	}
	writeTail(&b, out.Warnings, "mulch")
	return done(*asJSON, out, b.String())
}

func writeTail(b *strings.Builder, warnings []string, name string) {
	for _, w := range warnings {
		fmt.Fprintf(b, "Warning: %s\n", w)
	}
	fmt.Fprintf(b, "Planning quantities: the product label, the supplier and the site govern.\nMethod and sources: %s\n", page(name))
}

// ---- soil ----

type soilJSON struct {
	Calculator       string   `json:"calculator"`
	Page             string   `json:"page"`
	Material         string   `json:"material"`
	SqFt             float64  `json:"sq_ft"`
	CuFt             float64  `json:"cu_ft"`
	CuYd             float64  `json:"cu_yd"`
	OrderedCuFt      float64  `json:"ordered_cu_ft"`
	OrderedCuYd      float64  `json:"ordered_cu_yd"`
	Bags             *float64 `json:"bags,omitempty"`
	BagCost          *float64 `json:"bag_cost,omitempty"`
	BulkCuYd         float64  `json:"bulk_cu_yd"`
	LooseLb          float64  `json:"loose_lb"`
	BulkTons         float64  `json:"bulk_tons"`
	BulkCost         *float64 `json:"bulk_cost,omitempty"`
	Cheaper          string   `json:"cheaper,omitempty"`
	BankLb           *float64 `json:"bank_lb,omitempty"`
	SettledCuFt      *float64 `json:"settled_cu_ft,omitempty"`
	Trips            *float64 `json:"trips,omitempty"`
	WheelbarrowLoads *float64 `json:"wheelbarrow_loads,omitempty"`
	Warnings         []string `json:"warnings"`
}

func runSoil(args []string) (result, error) {
	fs, asJSON := newFlags("soil", "Topsoil or fill dirt: volume, bags, bulk, weight and trips. The material\nweight is yours to enter (supplier's ticket or reference of your choice).")
	shape := fs.String("shape", "rect", "rect, circle, triangle or area")
	length := fs.Float64("length", 0, "length, ft (rect)")
	width := fs.Float64("width", 0, "width, ft (rect)")
	diameter := fs.Float64("diameter", 0, "diameter, ft (circle)")
	base := fs.Float64("base", 0, "base, ft (triangle)")
	height := fs.Float64("height", 0, "height, ft (triangle)")
	area := fs.Float64("area", 0, "known area, sq ft (area)")
	count := fs.Float64("count", 1, "number of identical areas")
	depth := fs.Float64("depth", 0, "fill depth, in (required)")
	material := fs.String("material", "topsoil", "topsoil or fill")
	loose := fs.Float64("loose", 0, "loose weight, lb per cu yd (required: supplier's ticket)")
	bank := fs.Float64("bank", 0, "bank weight, lb per cu yd (optional)")
	loadFactor := fs.Float64("load-factor", 0, "load factor, loose to bank volume (optional)")
	bag := fs.Float64("bag", 0, "bag size, cu ft (0 for no bags)")
	bagPrice := fs.Float64("bag-price", 0, "price of one bag, USD")
	bulkPrice := fs.Float64("bulk-price", 0, "bulk price, USD per cu yd or per ton (see --bulk-by)")
	bulkBy := fs.String("bulk-by", "yard", "yard or ton")
	delivery := fs.Float64("delivery", 0, "delivery charge, USD")
	extra := fs.Float64("extra", 0, "extra, percent")
	payload := fs.Float64("payload", 0, "vehicle payload, lb (0 for none)")
	wheelbarrow := fs.Float64("wheelbarrow", 0, "wheelbarrow volume, cu ft (0 for none)")
	if err := parse(fs, args); err != nil {
		return result{}, err
	}
	var sh soil.Shape
	var req [][2]string
	switch *shape {
	case "rect":
		req = [][2]string{{"length", "the length in feet"}, {"width", "the width in feet"}}
		sh = soil.Rect{L: *length * ft, W: *width * ft}
	case "circle":
		req = [][2]string{{"diameter", "the diameter in feet"}}
		sh = soil.Circle{D: *diameter * ft}
	case "triangle":
		req = [][2]string{{"base", "the base in feet"}, {"height", "the height in feet"}}
		sh = soil.Triangle{B: *base * ft, H: *height * ft}
	case "area":
		req = [][2]string{{"area", "the area in square feet"}}
		sh = soil.KnownArea{Area: *area * sqft}
	default:
		return result{}, choice("shape", *shape, "rect", "circle", "triangle", "area")
	}
	req = append(req, [2]string{"depth", "the fill depth in inches"},
		[2]string{"loose", "the loose weight in lb per cu yd from the supplier's ticket; Calculiva supplies no default"})
	if err := need(fs, req...); err != nil {
		return result{}, err
	}
	if *loose <= 0 {
		return result{}, usageError("--loose must be greater than 0 lb per cu yd")
	}
	if *bank < 0 || *loadFactor < 0 {
		return result{}, usageError("--bank and --load-factor cannot be negative")
	}
	if err := choice("material", *material, "topsoil", "fill"); err != nil {
		return result{}, err
	}
	if err := choice("bulk-by", *bulkBy, "yard", "ton"); err != nil {
		return result{}, err
	}
	mat, by := soil.Topsoil, soil.Yard
	if *material == "fill" {
		mat = soil.Fill
	}
	if *bulkBy == "ton" {
		by = soil.Ton
	}
	const weightID = "entered"
	rules := soil.NewRules(soil.Weight{ID: weightID, Label: "entered by the user", Loose: *loose, Bank: *bank, LoadFactor: *loadFactor})
	mi := &soil.MaterialInput{BagSize: *bag, BagUnit: soil.CubicFeet, BagPrice: *bagPrice, BulkPrice: *bulkPrice, Weight: weightID}
	in := soil.Input{
		Areas:       []soil.Area{{ID: *shape, Shape: sh, Depth: *depth * inch, Count: *count, Material: mat}},
		BulkBy:      by,
		Delivery:    *delivery,
		Extra:       *extra,
		Payload:     *payload,
		Wheelbarrow: *wheelbarrow,
	}
	if mat == soil.Topsoil {
		in.Topsoil = mi
	} else {
		in.Fill = mi
	}
	plan, err := soil.NewPlan(in, rules)
	if err != nil {
		return result{}, err
	}
	m := plan.Materials[0]
	out := soilJSON{
		Calculator: "soil", Page: page("soil"), Material: m.ID.ID(),
		SqFt: plan.Areas[0].SqFt * plan.Areas[0].Count, CuFt: m.CuFt, CuYd: m.CuYd,
		OrderedCuFt: m.OrderedCuFt, OrderedCuYd: m.OrderedCuYd, Bags: m.Bags, BagCost: m.BagCost,
		BulkCuYd: m.BulkCuYd, LooseLb: m.LooseLb, BulkTons: m.BulkTons, BulkCost: m.BulkCost,
		Cheaper: string(m.Cheaper), Trips: m.Trips, WheelbarrowLoads: m.WheelbarrowLoads, Warnings: []string{},
	}
	if isSet(fs, "bank") {
		out.BankLb = &m.BankLb
	}
	if isSet(fs, "load-factor") {
		out.SettledCuFt = &m.SettledCuFt
	}
	for _, w := range plan.Warnings {
		out.Warnings = append(out.Warnings, fmt.Sprintf("sale increment adds %.1f cu ft of %s", w.SurplusCuFt, w.Material.ID()))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Soil, %s (%s): %.1f sq ft at %s in\n", out.Material, *shape, out.SqFt, num(*depth))
	fmt.Fprintf(&b, "Volume:  %.1f cu ft (%.2f cu yd)\n", out.CuFt, out.CuYd)
	fmt.Fprintf(&b, "Ordered: %.1f cu ft (%.2f cu yd) with %s %% extra\n", out.OrderedCuFt, out.OrderedCuYd, num(*extra))
	if out.Bags != nil {
		fmt.Fprintf(&b, "Bags:    %.0f x %s cu ft, %s\n", *out.Bags, num(*bag), money(out.BagCost))
	}
	fmt.Fprintf(&b, "Bulk:    %.2f cu yd, %.0f lb loose (%.2f tons), %s\n", out.BulkCuYd, out.LooseLb, out.BulkTons, money(out.BulkCost))
	if out.Cheaper != "" {
		fmt.Fprintf(&b, "Cheaper: %s\n", out.Cheaper)
	}
	if out.BankLb != nil {
		fmt.Fprintf(&b, "Bank weight: %.0f lb\n", *out.BankLb)
	}
	if out.SettledCuFt != nil {
		fmt.Fprintf(&b, "Settled: %.1f cu ft\n", *out.SettledCuFt)
	}
	if out.Trips != nil {
		fmt.Fprintf(&b, "Trips: %.0f\n", *out.Trips)
	}
	if out.WheelbarrowLoads != nil {
		fmt.Fprintf(&b, "Wheelbarrow loads: %.0f\n", *out.WheelbarrowLoads)
	}
	writeTail(&b, out.Warnings, "soil")
	return done(*asJSON, out, b.String())
}

// ---- sod ----

type sodJSON struct {
	Calculator string   `json:"calculator"`
	Page       string   `json:"page"`
	Format     string   `json:"format"`
	PieceWIn   float64  `json:"piece_w_in"`
	PieceLIn   float64  `json:"piece_l_in"`
	SqFt       float64  `json:"sq_ft"`
	Pieces     float64  `json:"pieces"`
	PerPallet  float64  `json:"per_pallet"`
	Pallets    float64  `json:"pallets"`
	Full       float64  `json:"full_pallets"`
	Loose      float64  `json:"loose_pieces"`
	Leftover   float64  `json:"leftover_pieces"`
	CostWhole  *float64 `json:"cost_whole_pallets,omitempty"`
	CostMixed  *float64 `json:"cost_mixed,omitempty"`
	Warnings   []string `json:"warnings"`
}

func runSod(args []string) (result, error) {
	rules := sod.NewRules()
	ids := make([]string, 0, len(rules.Formats)+1)
	for _, f := range rules.Formats {
		ids = append(ids, f.ID)
	}
	ids = append(ids, "custom")
	fs, asJSON := newFlags("sod", "Solid sod: pieces laid out in staggered rows on rectangles, by area with\nyour allowance elsewhere, then pallets and cost. The pallet size is yours to enter.")
	shape := fs.String("shape", "rect", "rect, circle, triangle or area")
	length := fs.Float64("length", 0, "length, ft (rect)")
	width := fs.Float64("width", 0, "width, ft (rect)")
	radius := fs.Float64("radius", 0, "radius, ft (circle)")
	base := fs.Float64("base", 0, "base, ft (triangle)")
	height := fs.Float64("height", 0, "height, ft (triangle)")
	area := fs.Float64("area", 0, "known area, sq ft (area)")
	count := fs.Float64("count", 1, "number of identical areas (rect, circle, triangle)")
	format := fs.String("format", rules.Formats[0].ID, "piece format: "+strings.Join(ids, ", "))
	pieceW := fs.Float64("piece-width", 0, "custom piece width, in (format custom)")
	pieceL := fs.Float64("piece-length", 0, "custom piece length, in (format custom)")
	palletPieces := fs.Float64("pallet-pieces", 0, "pieces per pallet (this or --pallet-sqft is required)")
	palletSqFt := fs.Float64("pallet-sqft", 0, "square feet per pallet (this or --pallet-pieces is required)")
	allowance := fs.Float64("allowance", 0, "allowance on areas without a layout, percent")
	spare := fs.Float64("spare", 0, "spare on laid-out rectangles, percent")
	price := fs.Float64("price", 0, "price, USD (see --price-per)")
	pricePer := fs.String("price-per", "pallet", "pallet, piece or sqft")
	delivery := fs.Float64("delivery", 0, "delivery charge, USD")
	if err := parse(fs, args); err != nil {
		return result{}, err
	}
	var sh sod.Shape
	var req [][2]string
	switch *shape {
	case "rect":
		req = [][2]string{{"length", "the length in feet"}, {"width", "the width in feet"}}
		sh = sod.Rect{L: *length * ft, W: *width * ft, Count: *count}
	case "circle":
		req = [][2]string{{"radius", "the radius in feet"}}
		sh = sod.Circle{R: *radius * ft, Count: *count}
	case "triangle":
		req = [][2]string{{"base", "the base in feet"}, {"height", "the height in feet"}}
		sh = sod.Triangle{B: *base * ft, H: *height * ft, Count: *count}
	case "area":
		req = [][2]string{{"area", "the area in square feet"}}
		sh = sod.KnownArea{Area: *area * sqft}
	default:
		return result{}, choice("shape", *shape, "rect", "circle", "triangle", "area")
	}
	if err := choice("format", *format, ids...); err != nil {
		return result{}, err
	}
	if *format == "custom" {
		req = append(req, [2]string{"piece-width", "the piece width in inches"}, [2]string{"piece-length", "the piece length in inches"})
	}
	if err := need(fs, req...); err != nil {
		return result{}, err
	}
	byPieces, bySqFt := isSet(fs, "pallet-pieces"), isSet(fs, "pallet-sqft")
	if byPieces == bySqFt {
		return result{}, usageError("missing required flag --pallet-pieces or --pallet-sqft (exactly one): the pallet size printed by your grower")
	}
	if err := choice("price-per", *pricePer, "pallet", "piece", "sqft"); err != nil {
		return result{}, err
	}
	in := sod.NewInput([]sod.Area{{ID: *shape, Shape: sh}}, rules)
	in.Format = *format
	in.PieceW, in.PieceL = *pieceW*inch, *pieceL*inch
	in.PalletPieces, in.PalletSqFt = *palletPieces, *palletSqFt
	if bySqFt {
		in.PalletMode = sod.PalletBySqFt
	}
	in.Allowance, in.Spare = *allowance, *spare
	in.Price, in.Delivery = *price, *delivery
	switch *pricePer {
	case "piece":
		in.PriceMode = sod.PricePerPiece
	case "sqft":
		in.PriceMode = sod.PricePerSqFt
	}
	plan, err := sod.NewPlan(in, rules)
	if err != nil {
		return result{}, err
	}
	out := sodJSON{
		Calculator: "sod", Page: page("sod"), Format: *format,
		PieceWIn: plan.PieceW / inch, PieceLIn: plan.PieceL / inch, SqFt: plan.Totals.SqFt,
		Pieces: plan.Totals.Pieces, PerPallet: plan.Pallets.PerPallet, Pallets: plan.Pallets.PalletsUp,
		Full: plan.Pallets.Full, Loose: plan.Pallets.Loose, Leftover: plan.Pallets.Leftover,
		Warnings: []string{},
	}
	if *price != 0 {
		out.CostWhole, out.CostMixed = plan.Cost.Whole, plan.Cost.Mixed
	}
	for _, w := range plan.Warnings {
		s := string(w.Kind)
		if w.Area != "" {
			s += " (" + w.Area + ")"
		}
		out.Warnings = append(out.Warnings, s)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Sod, %s: %.1f sq ft in %.4g x %.4g in pieces\n", *shape, out.SqFt, out.PieceWIn, out.PieceLIn)
	fmt.Fprintf(&b, "Pieces to buy: %.0f\n", out.Pieces)
	fmt.Fprintf(&b, "Pallets: %.0f of %.0f pieces (%.0f full + %.0f loose), %.0f left over\n", out.Pallets, out.PerPallet, out.Full, out.Loose, out.Leftover)
	if out.CostWhole != nil {
		fmt.Fprintf(&b, "Cost: %s in whole pallets, %s with loose pieces\n", money(out.CostWhole), money(out.CostMixed))
	}
	writeTail(&b, out.Warnings, "sod")
	return done(*asJSON, out, b.String())
}

// ---- asphalt ----

type asphaltJSON struct {
	Calculator string   `json:"calculator"`
	Page       string   `json:"page"`
	PCF        float64  `json:"pcf"`
	SqFt       float64  `json:"sq_ft"`
	SqYd       float64  `json:"sq_yd"`
	CuFt       float64  `json:"cu_ft"`
	CuYd       float64  `json:"cu_yd"`
	Tons       float64  `json:"tons"`
	Ordered    float64  `json:"ordered_tons"`
	Loads      float64  `json:"loads"`
	Delivered  float64  `json:"delivered_tons"`
	Surplus    float64  `json:"surplus_tons"`
	Cost       *float64 `json:"cost,omitempty"`
	Lifts      float64  `json:"lifts"`
	LiftIn     float64  `json:"lift_in"`
	SqFtPerTon float64  `json:"sq_ft_per_ton"`
	Warnings   []string `json:"warnings"`
}

func runAsphalt(args []string) (result, error) {
	fs, asJSON := newFlags("asphalt", "Hot mix asphalt for one zone and one course: tons, lifts and whole truck\nloads. The unit weight and the maximum lift are yours to enter (mix design,\npaving specification): Calculiva supplies no default.")
	shape := fs.String("shape", "rect", "rect, flare or round")
	width := fs.Float64("width", 0, "width, ft (rect; narrow width for flare)")
	width2 := fs.Float64("width2", 0, "wide width, ft (flare)")
	length := fs.Float64("length", 0, "length, ft (rect; depth of the flare)")
	radius := fs.Float64("radius", 0, "radius, ft (round)")
	part := fs.Float64("part", 1, "fraction of the disc: 1, 0.5 or 0.25 (round)")
	thickness := fs.Float64("thickness", 0, "compacted thickness, in (required)")
	nmas := fs.Float64("nmas", 0, "nominal maximum aggregate size, mm (0 if unknown)")
	minLift := fs.Float64("min-lift-multiple", 0, "minimum lift as a multiple of the aggregate size (required with --nmas)")
	pcf := fs.Float64("pcf", 0, "in-place unit weight, lb per cu ft (required, or the mix design flags)")
	rice := fs.Float64("rice", 0, "Rice value, Gmm (mix design)")
	compaction := fs.Float64("compaction", 0, "compaction, percent (mix design)")
	water := fs.Float64("water-pcf", 0, "unit weight of water, lb per cu ft (mix design)")
	maxLift := fs.Float64("max-lift", 0, "maximum lift for the roller, in (required)")
	roller := fs.String("roller", "static", "static or vibratory")
	extra := fs.Float64("extra", 0, "allowance, percent (0 to 25)")
	load := fs.Float64("load", 0, "truck load, tons (0 for none)")
	price := fs.Float64("price", 0, "price per ton, USD")
	if err := parse(fs, args); err != nil {
		return result{}, err
	}
	var sh asphalt.Shape
	var req [][2]string
	switch *shape {
	case "rect":
		req = [][2]string{{"width", "the width in feet"}, {"length", "the length in feet"}}
		sh = asphalt.Rect{W: *width * ft, L: *length * ft}
	case "flare":
		req = [][2]string{{"width", "the narrow width in feet"}, {"width2", "the wide width in feet"}, {"length", "the depth of the flare in feet"}}
		sh = asphalt.Flare{W: *width * ft, W2: *width2 * ft, L: *length * ft}
	case "round":
		req = [][2]string{{"radius", "the radius in feet"}}
		sh = asphalt.Round{R: *radius * ft, Part: *part}
	default:
		return result{}, choice("shape", *shape, "rect", "flare", "round")
	}
	req = append(req, [2]string{"thickness", "the compacted thickness in inches"})
	if err := need(fs, req...); err != nil {
		return result{}, err
	}
	var unit asphalt.UnitWeight
	mix := isSet(fs, "rice") || isSet(fs, "compaction") || isSet(fs, "water-pcf")
	switch {
	case isSet(fs, "pcf") && mix:
		return result{}, usageError("enter either --pcf or the mix design (--rice, --compaction, --water-pcf), not both")
	case isSet(fs, "pcf"):
		unit = asphalt.PCF(*pcf)
	case mix:
		if err := need(fs, [2]string{"rice", "the Rice value (Gmm) of the mix design"},
			[2]string{"compaction", "the compaction in percent"},
			[2]string{"water-pcf", "the unit weight of water in lb per cu ft of your reference"}); err != nil {
			return result{}, err
		}
		unit = asphalt.MixDesign{Rice: *rice, Compaction: *compaction}
	default:
		return result{}, usageError("missing required flag --pcf: the in-place unit weight in lb per cu ft from your mix design or plant (or --rice, --compaction and --water-pcf); Calculiva supplies no default")
	}
	if err := need(fs, [2]string{"max-lift", "the maximum lift for your roller in inches, from your paving specification"}); err != nil {
		return result{}, err
	}
	if isSet(fs, "nmas") && *nmas != 0 {
		if err := need(fs, [2]string{"min-lift-multiple", "the minimum lift as a multiple of the aggregate size, from your specification"}); err != nil {
			return result{}, err
		}
	}
	if err := choice("roller", *roller, "static", "vibratory"); err != nil {
		return result{}, err
	}
	rules := asphalt.NewRules()
	rules.WaterPCF = *water
	rules.MinLiftMultiple = *minLift
	in := asphalt.NewInput([]asphalt.Zone{{ID: *shape, Shape: sh}},
		[]asphalt.Course{{ID: "course", Thickness: *thickness * inch, NMAS: *nmas}}, unit)
	if *roller == "vibratory" {
		in.Roller = asphalt.Vibratory
		rules.MaxLiftVibratoryIn = *maxLift
	} else {
		rules.MaxLiftStaticIn = *maxLift
	}
	in.Extra, in.Load, in.Price = *extra, *load, *price
	plan, err := asphalt.NewPlan(in, rules)
	if err != nil {
		return result{}, err
	}
	c := plan.Courses[0]
	out := asphaltJSON{
		Calculator: "asphalt", Page: page("asphalt"), PCF: plan.PCF,
		SqFt: plan.SqFt, SqYd: plan.SqYd, CuFt: plan.CuFt, CuYd: plan.CuYd,
		Tons: plan.Tons, Ordered: plan.Ordered, Loads: plan.Loads, Delivered: plan.Delivered,
		Surplus: plan.Surplus, Lifts: c.Lifts, LiftIn: c.Lift / inch, SqFtPerTon: c.SqFtPerTon,
		Warnings: []string{},
	}
	if *price != 0 {
		out.Cost = &plan.Cost
	}
	for _, w := range plan.Warnings {
		out.Warnings = append(out.Warnings, string(w.Kind))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Asphalt, %s: %.1f sq ft (%.2f sq yd) at %s in, %s lb per cu ft\n", *shape, out.SqFt, out.SqYd, num(*thickness), num(out.PCF))
	fmt.Fprintf(&b, "Volume:  %.1f cu ft (%.2f cu yd)\n", out.CuFt, out.CuYd)
	fmt.Fprintf(&b, "Weight:  %.2f tons, %.2f ordered with %s %% allowance\n", out.Tons, out.Ordered, num(*extra))
	if out.Loads != 0 {
		fmt.Fprintf(&b, "Loads:   %.0f x %s tons = %.2f delivered (%.2f surplus)\n", out.Loads, num(*load), out.Delivered, out.Surplus)
	}
	if out.Cost != nil {
		fmt.Fprintf(&b, "Cost:    %s\n", money(out.Cost))
	}
	fmt.Fprintf(&b, "Lifts:   %.0f of %.2f in (%s roller)\n", out.Lifts, out.LiftIn, *roller)
	fmt.Fprintf(&b, "Coverage: %.1f sq ft per ton\n", out.SqFtPerTon)
	writeTail(&b, out.Warnings, "asphalt")
	return done(*asJSON, out, b.String())
}
