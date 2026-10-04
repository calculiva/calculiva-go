package asphalt

import (
	"errors"
	"math"
	"testing"

	calculiva "github.com/calculiva/calculiva-go"
)

const (
	in = 25.4
	ft = 304.8
)

// Synthetic test values, chosen for easy arithmetic and read on no publisher:
// the user enters the unit weight, the water unit weight and the lift rules.
const (
	synPCF         = 144.0
	synWaterPCF    = 60.0
	synMinLift     = 2.0
	synStaticIn    = 2.5
	synVibratoryIn = 6.0
)

// Reeves Construction calculator, value read in its script:
// https://reevescc.com/asphalt-calculator/ (Reeves Construction Company).
const reevesPCF = 150.0

func rules() Rules {
	r := NewRules()
	r.WaterPCF, r.MinLiftMultiple, r.MaxLiftStaticIn, r.MaxLiftVibratoryIn = synWaterPCF, synMinLift, synStaticIn, synVibratoryIn
	r.References = []Reference{
		{ID: "syn-low", Label: "synthetic low value", PCF: 140.0},
		{ID: "syn-mid", Label: "synthetic value equal to the job's", PCF: synPCF},
		{ID: "reeves", Label: "Reeves Construction calculator, value read in its script", PCF: reevesPCF, Publisher: "Reeves Construction Company", URL: "https://reevescc.com/asphalt-calculator/"},
	}
	return r
}

func near(t *testing.T, a, b, tol float64) {
	t.Helper()
	if !(math.Abs(a-b) <= tol) {
		t.Errorf("%v vs %v", a, b)
	}
}

func rect(w, l float64) Zone {
	return Zone{ID: "z1", Shape: Rect{W: w * ft, L: l * ft}}
}

func course(t, nmas float64, id string) Course {
	return Course{ID: id, Thickness: t * in, NMAS: nmas}
}

func job(zones []Zone, courses []Course, pcf float64) Input {
	return NewInput(zones, courses, PCF(pcf))
}

func plan(t *testing.T, c Input) *Plan {
	t.Helper()
	r, err := NewPlan(c, rules())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return r
}

func reference(t *testing.T, r *Plan, id string) ReferenceTons {
	t.Helper()
	for _, x := range r.References {
		if x.Reference.ID == id {
			return x
		}
	}
	t.Fatalf("reference %q not found", id)
	return ReferenceTons{}
}

func TestTonsOfA10By25FtSlab4InThick(t *testing.T) {
	r := plan(t, job([]Zone{rect(10.0, 25.0)}, []Course{course(4.0, 0.0, "c1")}, synPCF))
	near(t, r.CuFt, 83.333, 1e-3)
	near(t, r.Tons, 6.0, 1e-9)
}

func TestReproducesTheReevesCalculatorFor20By20FtAt2In(t *testing.T) {
	reeves := plan(t, job([]Zone{rect(20.0, 20.0)}, []Course{course(2.0, 0.0, "c1")}, reevesPCF))
	near(t, reeves.Tons, 5.0, 1e-9)
	near(t, reeves.CuYd, 2.469, 1e-3)
	near(t, reeves.CuFt, 66.667, 1e-3)
}

func TestLbPerSqYdPerInchAndBack(t *testing.T) {
	r := plan(t, job([]Zone{rect(9.0, 1.0)}, []Course{course(1.0, 0.0, "c1")}, 108.0*12.0/9.0))
	near(t, r.PCF, synPCF, 1e-9)
	near(t, r.LbPerSqYdIn, 108.0, 1e-9)
	near(t, r.Tons*2000.0, 108.0, 1e-6)
}

func TestUnitWeightFromTheMixDesignIsRiceTimesCompactionTimesWater(t *testing.T) {
	near(t, RicePCF(2.5, 96.0, synWaterPCF), synPCF, 1e-9)
	got, err := UnitWeightOf(MixDesign{Rice: 2.5, Compaction: 96.0}, synWaterPCF)
	if err != nil {
		t.Fatal(err)
	}
	near(t, got, synPCF, 1e-9)
	got, err = UnitWeightOf(MixDesign{Rice: 2.5, Compaction: 100.0}, synWaterPCF)
	if err != nil {
		t.Fatal(err)
	}
	near(t, got, 150.0, 1e-9)
}

func TestZoneShapesFlareIsATrapezoidRoundIsADiscFraction(t *testing.T) {
	z := func(shape Shape) float64 {
		a, err := ZoneArea(Zone{ID: "f", Shape: shape})
		if err != nil {
			t.Fatal(err)
		}
		return a / SqFt
	}
	near(t, z(Flare{W: 12.0 * ft, W2: 24.0 * ft, L: 10.0 * ft}), 180.0, 1e-6)
	near(t, z(Round{R: 10.0 * ft, Part: 0.5}), math.Pi*50.0, 1e-6)
	near(t, z(Round{R: 10.0 * ft, Part: 1.0}), math.Pi*100.0, 1e-6)
}

func TestTonsByZoneAddUpToTheCourseCoursesAddUpToTheJob(t *testing.T) {
	zones := []Zone{
		rect(12.0, 60.0),
		{ID: "z2", Shape: Flare{W: 12.0 * ft, W2: 24.0 * ft, L: 10.0 * ft}},
		{ID: "z3", Shape: Round{R: 15.0 * ft, Part: 0.5}},
	}
	r := plan(t, job(zones, []Course{course(2.5, 19.0, "base"), course(1.5, 9.5, "top")}, synPCF))
	near(t, r.SqFt, 720.0+180.0+math.Pi*112.5, 1e-6)
	for _, k := range r.Courses {
		sum := 0.0
		for _, z := range k.ByZone {
			sum += z.Tons
		}
		near(t, sum, k.Tons, 1e-9)
	}
	near(t, r.Courses[0].Tons+r.Courses[1].Tons, r.Tons, 1e-9)
	near(t, r.Tons, r.SqFt*(4.0/12.0)*synPCF/2000.0, 1e-9)
}

func TestLiftChecksUseTheRulesEntered(t *testing.T) {
	thin := plan(t, job([]Zone{rect(10.0, 10.0)}, []Course{course(1.5, 25.0, "c1")}, synPCF)).Courses[0]
	near(t, thin.MinLift/in, 50.0/25.4, 1e-9)
	if !thin.TooThin {
		t.Error("thin course not flagged")
	}
	if thin.Lifts != 1.0 {
		t.Errorf("thin lifts = %v, want 1", thin.Lifts)
	}
	ok := plan(t, job([]Zone{rect(10.0, 10.0)}, []Course{course(1.5, 9.5, "c1")}, synPCF)).Courses[0]
	if ok.TooThin {
		t.Error("fine course flagged thin")
	}
	static := job([]Zone{rect(10.0, 10.0)}, []Course{course(4.0, 28.0, "c1")}, synPCF)
	static.Roller = Static
	thick := plan(t, static)
	if thick.Courses[0].Lifts != 2.0 {
		t.Errorf("thick lifts = %v, want 2", thick.Courses[0].Lifts)
	}
	near(t, thick.Courses[0].Lift/in, 2.0, 1e-3)
	if !thick.Courses[0].TooThin {
		t.Error("thick course under a static roller not flagged thin")
	}
	if len(thick.Warnings) != 2 || thick.Warnings[0].Kind != "thin" || thick.Warnings[1].Kind != "split" {
		t.Errorf("warnings = %v, want thin then split", thick.Warnings)
	}
	vibratory := job([]Zone{rect(10.0, 10.0)}, []Course{course(4.0, 28.0, "c1")}, synPCF)
	vibratory.Roller = Vibratory
	vib := plan(t, vibratory).Courses[0]
	if vib.Lifts != 1.0 {
		t.Errorf("vibratory lifts = %v, want 1", vib.Lifts)
	}
	if vib.TooThin {
		t.Error("vibratory course flagged thin")
	}
	if got := plan(t, job([]Zone{rect(10.0, 10.0)}, []Course{course(2.5, 0.0, "c1")}, synPCF)).Courses[0].Lifts; got != 1.0 {
		t.Errorf("2.5 in lifts = %v, want 1", got)
	}
}

func TestOrderAllowanceWholeLoadsSurplusAndCost(t *testing.T) {
	c := job([]Zone{rect(10.0, 25.0)}, []Course{course(4.0, 0.0, "c1")}, synPCF)
	c.Extra, c.Load, c.Price = 5.0, 3.0, 100.0
	r := plan(t, c)
	near(t, r.Ordered, 6.0*1.05, 1e-9)
	if r.Loads != 3.0 {
		t.Errorf("loads = %v, want 3", r.Loads)
	}
	near(t, r.Delivered, 9.0, 1e-3)
	near(t, r.Surplus, 9.0-r.Ordered, 1e-9)
	near(t, r.Cost, 900.0, 1e-3)
	none := plan(t, job([]Zone{rect(10.0, 25.0)}, []Course{course(4.0, 0.0, "c1")}, synPCF))
	if none.Loads != 0.0 {
		t.Errorf("loads = %v, want 0", none.Loads)
	}
	near(t, none.Delivered, none.Tons, 1e-12)
	near(t, none.Cost, 0.0, 1e-3)
	exact := job([]Zone{rect(10.0, 30.0)}, []Course{course(4.0, 0.0, "c1")}, 150.0)
	exact.Load = 2.5
	if got := plan(t, exact).Loads; got != 3.0 {
		t.Errorf("loads = %v, want 3", got)
	}
}

func TestReferenceComparisonSpansTheValuesEntered(t *testing.T) {
	r := plan(t, job([]Zone{rect(20.0, 20.0)}, []Course{course(2.0, 0.0, "c1")}, synPCF))
	if len(r.References) != 3 {
		t.Fatalf("references = %d, want 3", len(r.References))
	}
	near(t, reference(t, r, "syn-mid").Delta, 0.0, 1e-12)
	near(t, r.Spread, (150.0-140.0)*66.6667/2000.0, 1e-3)
	if !(reference(t, r, "syn-low").Delta < 0.0) {
		t.Error("syn-low delta is not negative")
	}
	bare := rules()
	bare.References = nil
	p, err := NewPlan(job([]Zone{rect(20.0, 20.0)}, []Course{course(2.0, 0.0, "c1")}, synPCF), bare)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.References) != 0 || p.Spread != 0.0 {
		t.Errorf("references = %v, spread = %v, want none and 0", p.References, p.Spread)
	}
}

func TestCoveragePerTonAnswersThe2InchQuestion(t *testing.T) {
	c := CoverageTable(synPCF, []float64{1.0, 2.0, 3.0, 4.0}, PoundsPerShortTon)
	near(t, c[1].SqFtPerTon, 83.333, 1e-3)
	near(t, c[1].SqYdPerTon, 9.259, 1e-3)
	near(t, c[0].SqFtPerTon, 2.0*c[1].SqFtPerTon, 1e-9)
	near(t, plan(t, job([]Zone{rect(10.0, 10.0)}, []Course{course(2.0, 0.0, "c1")}, synPCF)).Courses[0].SqFtPerTon, 83.333, 1e-3)
}

func TestInvalidInputNamesItsField(t *testing.T) {
	badWith := func(c Input, r Rules, field string) {
		t.Helper()
		_, err := NewPlan(c, r)
		var ie *calculiva.InputError
		if !errors.As(err, &ie) {
			t.Errorf("%s: got %v, want an input error", field, err)
			return
		}
		if ie.Field != field {
			t.Errorf("field = %q, want %q", ie.Field, field)
		}
	}
	bad := func(c Input, field string) {
		t.Helper()
		badWith(c, rules(), field)
	}
	bad(job([]Zone{}, []Course{course(2.0, 0.0, "c1")}, synPCF), "zones")
	bad(job([]Zone{{ID: "z1", Shape: Rect{W: math.NaN(), L: 10.0 * ft}}}, []Course{course(2.0, 0.0, "c1")}, synPCF), "z1-w")
	bad(job([]Zone{rect(10.0, 10.0)}, []Course{{ID: "c1", Thickness: 0.0, NMAS: 0.0}}, synPCF), "c1-thickness")
	bad(job([]Zone{rect(10.0, 10.0)}, []Course{course(2.0, 0.0, "c1")}, 60.0), "pcf")
	bad(NewInput([]Zone{rect(10.0, 10.0)}, []Course{course(2.0, 0.0, "c1")}, nil), "unit")
	bad(NewInput([]Zone{rect(10.0, 10.0)}, []Course{course(2.0, 0.0, "c1")}, MixDesign{Rice: 2.5, Compaction: 50.0}), "compaction")
	bad(job([]Zone{{ID: "z9", Shape: Round{R: 10.0 * ft, Part: 0.3}}}, []Course{course(2.0, 0.0, "c1")}, synPCF), "z9-part")
	half := job([]Zone{rect(10.0, 10.0)}, []Course{course(2.0, 0.0, "c1")}, synPCF)
	half.Load = 0.5
	bad(half, "load")
	// Without the user's entries, the rules carry no value to fall back on.
	empty := NewRules()
	badWith(NewInput([]Zone{rect(10.0, 10.0)}, []Course{course(2.0, 0.0, "c1")}, MixDesign{Rice: 2.5, Compaction: 96.0}), empty, "water")
	badWith(job([]Zone{rect(10.0, 10.0)}, []Course{course(2.0, 0.0, "c1")}, synPCF), empty, "max-lift")
	noMin := rules()
	noMin.MinLiftMultiple = 0.0
	badWith(job([]Zone{rect(10.0, 10.0)}, []Course{course(2.0, 19.0, "c1")}, synPCF), noMin, "min-lift")
}
