package getquote

import "testing"

// go test
func TestGetQuote(t *testing.T) {
	quote := GetQuote()
	if len(quote) == 0 {
		t.Errorf("Problem getting quote")
	}
}

// go test -bench=.
func BenchmarkGetQuote(b *testing.B) {
	for n := 0; n < b.N; n++ {
		GetQuote()
	}
}
