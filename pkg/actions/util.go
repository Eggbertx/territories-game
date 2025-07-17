package actions

import (
	"math"
	"math/rand"
	"testing"
)

func randInt(max int) int {
	if testing.Testing() && useTestInt {
		return int(math.Min(float64(testInt), float64(max))) - 1
	}
	return rand.Intn(max)
}
