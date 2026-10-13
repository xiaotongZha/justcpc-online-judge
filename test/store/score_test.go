package store_test

import (
	"testing"

	"justcpc-online-judge/src/store"
)

func TestScoreFormattingAndParsing(t *testing.T) {
	for _, test := range []struct {
		input string
		cents int64
		text  string
	}{{"100", 10000, "100"}, {"12.5", 1250, "12.5"}, {"12.01", 1201, "12.01"}} {
		cents, err := store.ParseScore(test.input)
		if err != nil || cents != test.cents {
			t.Fatalf("ParseScore(%q) = %d, %v", test.input, cents, err)
		}
		if got := store.FormatScore(cents); got != test.text {
			t.Fatalf("FormatScore(%d) = %q, want %q", cents, got, test.text)
		}
	}
	for _, invalid := range []string{"", "1.000", "-1", "1.", "a"} {
		if _, err := store.ParseScore(invalid); err == nil {
			t.Fatalf("ParseScore(%q) unexpectedly succeeded", invalid)
		}
	}
}
