package power

import "testing"

func TestValidate(t *testing.T) {
	for _, percent := range []int{0, 50, 100} {
		if err := Validate(Reading{Percent: percent}); err != nil {
			t.Fatalf("Validate(%d): %v", percent, err)
		}
	}
	for _, percent := range []int{-1, 101} {
		if err := Validate(Reading{Percent: percent}); err == nil {
			t.Fatalf("Validate(%d) unexpectedly succeeded", percent)
		}
	}
}
