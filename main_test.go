package main

import "testing"

func BenchmarkXYToRGB(b *testing.B) {
	for b.Loop() {
		_, _, _ = xyToRGB(0.3127, 0.3290, 0.5)
	}
}
