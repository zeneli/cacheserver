package rangecache

import (
	"testing"
	"time"
)

var incomingRanges = []struct {
	description   string
	keyrangeToAdd []Keyrange
	keyrangeToGet []Keyrange
	expectedOk    []bool // corresponds to the keyrangeToGet slice
}{
	{"exact range match",
		[]Keyrange{{0, 100}, {50, 75}, {75, 100}},
		[]Keyrange{{0, 100}, {50, 75}, {75, 100}},
		[]bool{true, true, true},
	},
	{"range overlap: range lies completely inside existing range",
		[]Keyrange{{0, 100}},
		[]Keyrange{{50, 75}, {75, 100}},
		[]bool{true, true},
	},
}

func TestGet(t *testing.T) {
	for _, tt := range incomingRanges {
		rc := New()

		//log.Println(tt.description)
		// Add the key ranges to the range cache
		for _, kr := range tt.keyrangeToAdd {
			//start := time.Now()
			rc.Add(kr, generateValue(kr))
			//log.Printf("%s, Add(%v)\n", time.Since(start), kr)
		}

		// Get the key ranges from the range cache
		for i, kr := range tt.keyrangeToGet {
			//start := time.Now()
			_, ok := rc.Get(kr)
			if ok != tt.expectedOk[i] {
				t.Fatalf("%s: range cache hit is %v, want %v", tt.description, ok, !ok)
			}
			//log.Printf("%s, Get(%v)\n", time.Since(start), kr)
		}
		//log.Println()
	}
}

// generateValue is a helper function that associats the value of a
// key range to a sequence of integers
// from start to end, inclusive.
func generateValue(kr Keyrange) []int {
	total := kr.End - kr.Start
	start := kr.Start

	krValue := make([]int, total+1)
	for i := 0; i <= total; i++ {
		krValue[i] = start + i
		time.Sleep(10 * time.Microsecond)
	}
	return krValue
}
