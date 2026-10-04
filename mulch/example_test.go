package mulch_test

import (
	"fmt"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/mulch"
)

// Two 40 x 3 ft beds topped up from 1 in to 3 in, bought in 2 cu ft bags.
func ExampleNewPlan() {
	bed := mulch.Area{
		ID:       "front-bed",
		Kind:     mulch.Bed{L: 40 * calculiva.Foot, W: 3 * calculiva.Foot, Count: 2},
		Target:   3 * calculiva.Inch,
		Existing: 1 * calculiva.Inch,
	}
	input := mulch.Input{Areas: []mulch.Area{bed}, BagSize: 2}

	plan, err := mulch.NewPlan(input, mulch.NewRules())
	if err != nil {
		fmt.Println(err)
		return
	}
	// 240 sq ft at 2 in.
	fmt.Printf("%.1f cu ft, %.0f bags\n", plan.Totals.CuFt, *plan.Buy.Bags)
	// Output: 40.0 cu ft, 20 bags
}
