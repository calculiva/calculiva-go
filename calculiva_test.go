package calculiva

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryHasItsHeader(t *testing.T) {
	const header = "id,label,status,scope,source_name,source_url,verified_at,expires_at,sample_size"
	if !strings.HasPrefix(ConstantsCSV(), header) {
		t.Fatalf("registry does not start with %q", header)
	}
}

func TestEmbeddedRegistryMatchesTheSiteCopy(t *testing.T) {
	original := filepath.Join("..", "..", "site", "public", "data", "calculiva-home-improvement-constants.csv")
	text, err := os.ReadFile(original)
	if err != nil {
		t.Skipf("%s not found (module outside its source tree)", original)
	}
	if ConstantsCSV() != string(text) {
		t.Fatalf("module copy differs from %s", original)
	}
}

func TestUnitsAreMillimetres(t *testing.T) {
	if Inch != 25.4 || Foot != 304.8 {
		t.Fatalf("Inch = %v, Foot = %v", Inch, Foot)
	}
}

func TestInputErrorNamesItsField(t *testing.T) {
	var err error = &InputError{Message: "Enter a width.", Field: "z1-w"}
	if got := err.Error(); got != "Enter a width. (z1-w)" {
		t.Fatalf("Error() = %q", got)
	}
	var ie *InputError
	if !errors.As(err, &ie) || ie.Field != "z1-w" {
		t.Fatalf("errors.As did not recover the field")
	}
}
