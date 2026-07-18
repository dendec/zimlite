package ui

import "testing"

func TestAnalogDirection(t *testing.T) {
	tests := []struct {
		name      string
		value     int16
		direction int32
		strength  float64
	}{
		{name: "deadzone", value: analogDeadZone, direction: 0, strength: 0},
		{name: "up", value: -32767, direction: -1, strength: 1},
		{name: "down", value: 32767, direction: 1, strength: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			direction, strength := analogDirection(test.value)
			if direction != test.direction {
				t.Fatalf("direction = %d, want %d", direction, test.direction)
			}
			if strength != test.strength {
				t.Fatalf("strength = %f, want %f", strength, test.strength)
			}
		})
	}
}

func TestAnalogDirectionStrengthIncreasesWithDeflection(t *testing.T) {
	_, low := analogDirection(analogDeadZone + 1000)
	_, high := analogDirection(analogDeadZone + 10000)
	if low <= 0 || high <= low || high >= 1 {
		t.Fatalf("strengths = %f, %f; want 0 < low < high < 1", low, high)
	}
}
