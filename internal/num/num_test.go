package num

import "testing"

func TestLocaleFormatsLikeJavaScript(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{Locale(2438.4, 1), "2,438.4"},
		{Locale(8.0, 1), "8"},
		{Locale(91.44, 1), "91.4"},
		{JSNum(60000.0), "60000"},
		{JSNum(0.5), "0.5"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
