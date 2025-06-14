package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
import numpy as np

class MarkedHawkesProcessVP:
    def __init__(self, mu=0.1, alpha=0.01, beta=1.0):
        self.alpha = alpha  # Intensity of the process
        self.mu = mu        # Baseline intensity
        self.beta = beta  # Marks associated with the process


    def mark_function(self, volume, price):
        return volume * price

    def intensity(self, t, history):
        past_events = [(ti, vi, pi) for ti, vi, pi in history if ti < t]
        excitation = sum(self.alpha * self.mark_function(vi, pi) *\
                np.exp(-self.beta * (t - ti)) for ti, vi, pi in past_events)
        return self.mu + excitation

mu = 0.5
alpha = 0.1
beta = 1.0
process = MarkedHawkesProcessVP(mu, alpha, beta)
intensity = process.intensity(3, [(1, 100, 10), (2, 200, 20), (3, 300, 30), (4, 400, 40)])

# result: 161.1853047922382
*/

func TestMarkedHawkesProcessVP(t *testing.T) {
	mu := 0.5
	alpha := 0.1
	beta := 1.0

	process := NewMarkedHawkesProcessVP(mu, alpha, beta, 1.0)
	history := []TimedTuple{
		{Time: 1, Params: VolumePriceData{Volume: 100, Price: 10}},
		{Time: 2, Params: VolumePriceData{Volume: 200, Price: 20}},
		{Time: 3, Params: VolumePriceData{Volume: 300, Price: 30}},
		{Time: 4, Params: VolumePriceData{Volume: 400, Price: 40}},
	}

	intensity := GetIntensity(process, 3, history)

	expectedIntensity := 161.1853047922382
	if intensity != expectedIntensity {
		t.Errorf("Expected intensity %f, got %f", expectedIntensity, intensity)
	}
}

func TestMarkedHawkesInvalidParam(t *testing.T) {
	process := NewMarkedHawkesProcessVP(0.5, 0.1, 1.0, 1.0)
	result := process.MarkFunction(199)
	assert.Equal(t, 0.0, result, "Expected mark function to return 0 for invalid input")
}
