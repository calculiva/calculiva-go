// Package num holds the number helpers shared by the estimators: tolerant
// rounding, range checks and the number formats used in error messages.
package num

import (
	"math"
	"strconv"
	"strings"

	calculiva "github.com/calculiva/calculiva-go"
)

// Eps is the absolute tolerance of the range checks, in the unit of the value.
const Eps = 1e-7

// Bounds of a plan length, mm, and the end of the message that states them
// (soil, mulch).
const (
	LenMin = 50.0
	LenMax = 60000.0
	LenMsg = " from 50 mm to 60 m (about 2 in to 196 ft)."
)

// epsilon is the difference between 1 and the next float64.
const epsilon = 1.0 / (1 << 52)

// NewError builds an input error for one field.
func NewError(message, field string) error {
	return &calculiva.InputError{Message: message, Field: field}
}

// Locale formats a number the way JavaScript's
// toLocaleString('en-US', {maximumFractionDigits}) does for the values used in
// messages: grouped thousands, trailing zeros removed.
func Locale(v float64, digits int) string {
	s := strconv.FormatFloat(v, 'f', digits, 64)
	whole, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		whole, frac = s[:i], strings.TrimRight(s[i+1:], "0")
	}
	neg := strings.HasPrefix(whole, "-")
	body := strings.TrimLeft(whole, "-")
	var grouped strings.Builder
	for i := 0; i < len(body); i++ {
		if i > 0 && (len(body)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteByte(body[i])
	}
	var out strings.Builder
	if neg && (strings.Trim(grouped.String(), "0,") != "" || frac != "") {
		out.WriteByte('-')
	}
	out.WriteString(grouped.String())
	if frac != "" {
		out.WriteByte('.')
		out.WriteString(frac)
	}
	return out.String()
}

// JSNum formats a number the way JavaScript's String(n) does for the values
// used in messages.
func JSNum(v float64) string {
	if !math.IsInf(v, 0) && !math.IsNaN(v) && v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// CeilRel rounds up, forgiving a relative 1e-9 of floating-point noise
// (soil, mulch, sod).
func CeilRel(n float64) float64 {
	return math.Ceil(n - float64(1e-9*math.Max(math.Abs(n), 1)))
}

// FloorRel rounds down, forgiving a relative 1e-9 of floating-point noise (sod).
func FloorRel(n float64) float64 {
	return math.Floor(n + float64(1e-9*math.Max(math.Abs(n), 1)))
}

// CeilULP rounds up, forgiving eight ULPs (asphalt).
func CeilULP(n float64) float64 {
	return math.Ceil(n - float64(epsilon*math.Max(n, 1)*8))
}

// Range checks that v is a finite number within [min, max] (tolerance 1e-7).
func Range(v, min, max float64, message, field string) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < min-Eps || v > max+Eps {
		return NewError(message, field)
	}
	return nil
}
