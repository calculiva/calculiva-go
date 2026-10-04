package sod_test

import (
	"fmt"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/sod"
)

// A 25 x 18 ft lawn laid in 16 x 24 in slabs sold 170 to the pallet.
func ExampleNewPlan() {
	rules := sod.NewRules()
	lawn := sod.Area{ID: "lawn", Shape: sod.Rect{L: 25 * calculiva.Foot, W: 18 * calculiva.Foot, Count: 1}}
	input := sod.NewInput([]sod.Area{lawn}, rules)
	input.Format = "slab-16x24"
	input.PalletPieces = 170

	plan, err := sod.NewPlan(input, rules)
	if err != nil {
		fmt.Println(err)
		return
	}
	layout := plan.Areas[0].Layout
	fmt.Printf("%.0f sq ft in %d rows\n", plan.Totals.SqFt, len(layout.Rows))
	fmt.Printf("%.0f pieces to buy for %d placed\n", plan.Totals.Pieces, layout.Installed)
	fmt.Printf("%.0f pallet, %.0f piece left over\n", plan.Pallets.PalletsUp, plan.Pallets.Leftover)
	// Output:
	// 450 sq ft in 14 rows
	// 169 pieces to buy for 182 placed
	// 1 pallet, 1 piece left over
}
