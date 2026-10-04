# calculiva-go

Home improvement estimators from [calculiva.com](https://calculiva.com/), as a Go module:
mulch, topsoil and fill, sod and hot mix asphalt.

This module is a line-by-line port of the engine that runs the calculators on the site. The
test vectors of the site are reproduced as Go tests, with the same inputs, the same expected
outputs and the same tolerances, except where a value of a commercial publisher was replaced
by a synthetic test value labelled as such.

```
go get github.com/calculiva/calculiva-go
```

Go 1.21 or later. No dependency outside the standard library.

## What this module is, and is not

Calculiva is a data house, not a trade. Nobody here claims to be a landscaper or a paver.
What the module offers is arithmetic you can check and constants you can trace:

- every business constant shipped (a pallet size, a CPSC depth, a sale unit) is a named Go
  constant or variable whose documentation gives the id of its data sheet, the public source
  it was read on and the date it was verified;
- the values of commercial publishers whose terms do not allow reproduction (material and
  asphalt unit weights, bag and wheelbarrow sizes, sod allowances, asphalt lift rules) are not
  shipped: they are inputs of the user, with no default. The sources Calculiva read are listed
  at <https://calculiva.com/data-sources/>;
- nothing is rounded or invented to fill a gap: when a source is silent, the value is absent
  (a nil pointer or a nil slice, never a placeholder number);
- the results are planning quantities. The manufacturer's instructions, the local code and
  the installer on site govern.

All lengths are millimetres (`calculiva.Inch = 25.4`, `calculiva.Foot = 304.8`). Invalid input
returns a `*calculiva.InputError` that names the offending field.

## Example

```go
package main

import (
	"fmt"

	calculiva "github.com/calculiva/calculiva-go"
	"github.com/calculiva/calculiva-go/mulch"
)

func main() {
	// Two 40 x 3 ft beds topped up from 1 in to 3 in, bought in 2 cu ft bags.
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
	// 40.0 cu ft, 20 bags
}
```

## Packages and functions

| Function | What it does | Calculator |
|---|---|---|
| `mulch.NewRules` | Builds the mulch rules: Penn State maximum depths, CPSC 324/325 playground rules. | [Mulch calculator](https://calculiva.com/landscaping/mulch-calculator/) |
| `mulch.NewPlan` | Volume per bed, tree ring, known area, play structure or swing; bags, bulk, break-even, loads, warnings. | [Mulch calculator](https://calculiva.com/landscaping/mulch-calculator/) |
| `mulch.NewRingGuidance` | Published trunk gaps and minimum ring radii, publisher by publisher. | [Mulch calculator](https://calculiva.com/landscaping/mulch-calculator/) |
| `mulch.CheckRing` | A tree ring against every published gap and radius. | [Mulch calculator](https://calculiva.com/landscaping/mulch-calculator/) |
| `mulch.Coverage` | Square feet per bag at each depth, bags per cubic yard. | [Mulch calculator](https://calculiva.com/landscaping/mulch-calculator/) |
| `soil.NewRules` | Builds the soil rules: NIST sale units, with the loose and bank weights entered by the user. | [Soil calculator](https://calculiva.com/landscaping/soil-calculator/) |
| `soil.NewPlan` | Topsoil and fill by area: volume, bags, bulk by the yard or the ton, weight, trips, wheelbarrow loads. | [Soil calculator](https://calculiva.com/landscaping/soil-calculator/) |
| `soil.AreaOf` | Area of a rectangle, circle, triangle or known area. | [Soil calculator](https://calculiva.com/landscaping/soil-calculator/) |
| `soil.BagCuFt` | Bag volume in cubic feet from cubic feet, U.S. dry quarts or litres. | [Soil calculator](https://calculiva.com/landscaping/soil-calculator/) |
| `soil.Coverage` | Square feet covered per cubic yard, per ton and per cubic foot at each depth. | [Soil calculator](https://calculiva.com/landscaping/soil-calculator/) |
| `sod.NewRules` | Builds the sod rules: surveyed formats and pallets, soil preparation, UF/IFAS plugs; the allowances are entered by the user. | [Sod calculator](https://calculiva.com/landscaping/sod-calculator/) |
| `sod.NewPlan` | Pieces per area (rectangles laid out piece by piece), pallets, plugs, soil preparation, cost, weight. | [Sod calculator](https://calculiva.com/landscaping/sod-calculator/) |
| `sod.LayRect` | Staggered layout of one rectangle with offcut pairing and strip grouping. | [Sod calculator](https://calculiva.com/landscaping/sod-calculator/) |
| `sod.FormatTable` | Pieces per pallet coverage and per lawn for each format. | [Sod calculator](https://calculiva.com/landscaping/sod-calculator/) |
| `asphalt.NewRules` | Builds the asphalt rules: pounds per short ton (NIST); the water unit weight, lift rules and references are entered by the user. | [Asphalt calculator](https://calculiva.com/landscaping/asphalt-calculator/) |
| `asphalt.NewPlan` | Tons by zone and course, lifts under the roller, whole loads, cost, reference spread. | [Asphalt calculator](https://calculiva.com/landscaping/asphalt-calculator/) |
| `asphalt.ZoneArea` | Area of a rectangle, flare (trapezoid) or disc fraction. | [Asphalt calculator](https://calculiva.com/landscaping/asphalt-calculator/) |
| `asphalt.UnitWeightOf` | In-place unit weight (a required input), entered or from the Rice value and compaction. | [Asphalt calculator](https://calculiva.com/landscaping/asphalt-calculator/) |
| `asphalt.RicePCF` | Rice value (maximum theoretical specific gravity) x compaction x unit weight of water, all entered by the user. | [Asphalt calculator](https://calculiva.com/landscaping/asphalt-calculator/) |
| `asphalt.CoverageTable` | Square feet and yards covered by one ton at each thickness. | [Asphalt calculator](https://calculiva.com/landscaping/asphalt-calculator/) |
| `calculiva.ConstantsCSV` | The constants registry (one row per data sheet), embedded. | [Data sources](https://calculiva.com/data-sources/) |

## Data and licences

The code is under the MIT licence (`LICENSE`). The embedded registry
`data/calculiva-home-improvement-constants.csv`, returned by `calculiva.ConstantsCSV`, is under
Creative Commons Attribution 4.0 (`data/LICENSE`); credit "Calculiva" with a link to
<https://calculiva.com/>.

Market surveys expire: each such constant documents its expiry date. Check it before relying
on a price-sensitive or product-specific value.
