package calculiva

import _ "embed"

// Inch is one inch in millimetres (exact by definition of the international inch).
const Inch float64 = 25.4

// Foot is one foot in millimetres (exact by definition of the international foot).
const Foot float64 = 304.8

// InputError is an invalid input, with the name of the field that caused it.
type InputError struct {
	// Message is human-readable, in English.
	Message string
	// Field identifies the offending field (for example "a1-depth" or "bag").
	Field string
}

// Error returns the message followed by the field in parentheses.
func (e *InputError) Error() string {
	return e.Message + " (" + e.Field + ")"
}

//go:embed data/calculiva-home-improvement-constants.csv
var constantsCSV string

// ConstantsCSV returns the Calculiva home improvement constants registry
// (CSV, UTF-8), licensed CC BY 4.0.
//
// One row per data sheet: id, label, status, scope, source_name, source_url,
// verified_at, expires_at, sample_size.
func ConstantsCSV() string {
	return constantsCSV
}
