package asphalt_test

import (
	"fmt"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/asphalt"
)

// A 10 x 25 ft driveway paved 4 in thick at 150 lb per cubic foot, delivered in
// 3-ton loads. The unit weight and the maximum lift (2.5 in here) are example
// entries: take yours from the mix design and the paving specification.
func ExampleNewPlan() {
	input := asphalt.NewInput(
		[]asphalt.Zone{{ID: "driveway", Shape: asphalt.Rect{W: 10 * calculiva.Foot, L: 25 * calculiva.Foot}}},
		[]asphalt.Course{{ID: "surface", Thickness: 4 * calculiva.Inch}},
		asphalt.PCF(150),
	)
	input.Load = 3

	rules := asphalt.NewRules()
	rules.MaxLiftStaticIn = 2.5

	plan, err := asphalt.NewPlan(input, rules)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.1f cu ft, %.2f tons, %.0f loads, %.0f lifts\n", plan.CuFt, plan.Tons, plan.Loads, plan.Courses[0].Lifts)
	// Output: 83.3 cu ft, 6.25 tons, 3 loads, 2 lifts
}
