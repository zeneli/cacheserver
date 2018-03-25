package rangecache

import "testing"

var incomingRanges = []struct {
	description   string
	keyrangeToAdd []Keyrange
	keyrangeToGet []Keyrange
	expectedOk    []bool // corresponds to the keyrangeToGet slice
}{
	{"same found", []Keyrange{{0, 100}}, []Keyrange{{50, 75}, {75, 100}}, []bool{true, true}},
}

func TestGet(t *testing.T) {
	for _, tt := range incomingRanges {
		rc := New()

		// Add the key ranges to the range cache
		for _, kr := range tt.keyrangeToAdd {
			rc.Add(kr, generateValue(kr))
		}

		// Get the key ranges from the range cache
		for i, kr := range tt.keyrangeToGet {
			_, ok := rc.Get(kr)
			if ok != tt.expectedOk[i] {
				t.Fatalf("%s: range cache hit is %v, want %v", tt.description, ok, !ok)
			}
		}
	}
}

func generateValue(kr Keyrange) []int {
	total := kr.End - kr.Start
	start := kr.Start

	krValue := make([]int, total+1)
	for i := 0; i <= total; i++ {
		krValue[i] = start + i
	}
	return krValue
}
