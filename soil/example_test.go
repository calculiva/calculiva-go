package soil_test

import (
	"fmt"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/soil"
)

// Three 8 x 4 ft raised beds filled 10 in deep with topsoil, bought in 1 cu ft
// bags and moved with a 5 cu ft wheelbarrow. The wheelbarrow volume and the
// material weight are example entries: take yours from the tool and the
// supplier's ticket.
func ExampleNewPlan() {
	input := soil.Input{
		Areas: []soil.Area{{
			ID:       "beds",
			Shape:    soil.Rect{L: 8 * calculiva.Foot, W: 4 * calculiva.Foot},
			Depth:    10 * calculiva.Inch,
			Count:    3,
			Material: soil.Topsoil,
		}},
		Topsoil:     &soil.MaterialInput{BagSize: 1, BagUnit: soil.CubicFeet, Weight: "topsoil"},
		Wheelbarrow: 5,
	}
	rules := soil.NewRules(soil.Weight{ID: "topsoil", Label: "Topsoil, supplier's ticket", Loose: 2000, Bank: 2500, LoadFactor: 0.75})

	plan, err := soil.NewPlan(input, rules)
	if err != nil {
		fmt.Println(err)
		return
	}
	topsoil := plan.Materials[0]
	fmt.Printf("%.1f cu ft, %.2f cu yd\n", topsoil.CuFt, topsoil.CuYd)
	fmt.Printf("%.0f bags, %.0f wheelbarrow loads\n", *topsoil.Bags, *topsoil.WheelbarrowLoads)
	// Output:
	// 80.0 cu ft, 2.96 cu yd
	// 80 bags, 16 wheelbarrow loads
}
