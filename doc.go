// Package calculiva holds what the Calculiva home improvement estimators share:
// the length units, the input error type and the embedded constants registry.
//
// The estimators themselves live in the sub-packages mulch, soil, sod and asphalt.
// They are a line-by-line port of the engine that runs the calculators on
// https://calculiva.com/ and their tests reproduce the test vectors of the site,
// with the same tolerances, except where a value of a commercial publisher was
// replaced by a synthetic test value labelled as such.
//
// Calculiva is a data house, not a trade. Nobody here claims to be a landscaper
// or a paver. What the module offers is arithmetic you can check and constants
// you can trace:
//
//   - every business constant shipped (a pallet size, a CPSC depth, a sale unit)
//     is a named Go constant or variable whose documentation gives the id of its
//     data sheet, the public source it was read on and the date it was verified;
//   - the values of commercial publishers whose terms do not allow reproduction
//     (material and asphalt unit weights, bag and wheelbarrow sizes, sod
//     allowances, asphalt lift rules) are not shipped: they are inputs of the
//     user, with no default. The sources Calculiva read are listed at
//     https://calculiva.com/data-sources/;
//   - nothing is rounded or invented to fill a gap: when a source is silent, the
//     value is absent;
//   - the results are planning quantities. The manufacturer's instructions, the
//     local code and the installer on site govern.
//
// All lengths are millimetres ([Inch] = 25.4, [Foot] = 304.8). Invalid input
// returns an [*InputError] that names the offending field.
//
// The data sheets behind the constants are listed at
// https://calculiva.com/data-sources/ and the registry itself is returned by
// [ConstantsCSV].
package calculiva
