package lifecycle

import "testing"

func TestBootChanged(t *testing.T) {
	cases := []struct {
		previous string
		current  string
		changed  bool
	}{
		{"windows:1", "windows:2", true},
		{"linux:a", "linux:b", true},
		{"windows:1", "windows:1", false},
		{"{legacy-guid}", "windows:2", false},
		{"linux:a", "windows:2", false},
		{"", "windows:2", false},
	}
	for _, test := range cases {
		if got := BootChanged(test.previous, test.current); got != test.changed {
			t.Errorf("BootChanged(%q, %q) = %t, want %t", test.previous, test.current, got, test.changed)
		}
	}
}
