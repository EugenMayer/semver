package semver

import (
	"reflect"
	"testing"
)

func TestHyphenReplace(t *testing.T) {
	re := getRegex()
	tests := []struct {
		i string
		o string
	}{
		{">1.2.3", ">1.2.3"},
		{"1.2 - 3.4.5", ">=1.2.0 <=3.4.5"},
		{"1.2.3 - 3.4", ">=1.2.3 <3.5.0"},
		{"1.2 - 3.4", ">=1.2.0 <3.5.0"},
	}

	for _, tc := range tests {
		o := hyphenReplace(re, tc.i)
		if !reflect.DeepEqual(tc.o, o) {
			t.Errorf("Invalid for case %q: Expected %q, got: %q", tc.i, tc.o, o)
		}
	}
}
