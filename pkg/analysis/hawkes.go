package analysis

import (
	"math"
)

/*
reference: https://hawkeslib.readthedocs.io/en/latest/tutorial.html

Hawkes process is a self-exciting point process where the intensity of the process
is influenced by past events. The intensity function can be defined as:
	λ(t) = μ + ∑_{t_i < t} α * m(t_i) * exp(-β * (t - t_i) / τ)
	where:
	- λ(t) is the intensity at time t
	- μ is the baseline intensity
	- α is the excitation parameter
	- m(t_i) is the mark function evaluated at time t_i
	- β is the decay parameter
	- τ is the time scale parameter
The Hawkes process can be used to model various phenomena,
such as financial transactions, earthquakes, or any event-driven processes
where past events influence future occurrences.

To get the optimized Hawkes model parameters from history,
we need to maximize the log-likelihood function:

```python
from scipy.optimize import minimize

initial_guess = [0.1, 0.01, 1.0]  # mu, alpha, beta
result = minimize(log_likelihood, initial_guess, args=(history, T), method='L-BFGS-B',
                  bounds=[(1e-6, None), (0.0, None), (1e-6, None)])
```
*/

type TimedTuple struct {
	Time   int64 // Time of the event
	Params any
}

type MarkedHawkesProcess interface {
	MarkFunction(params any) float64
	GetAlpha() float64
	GetBeta() float64
	GetMu() float64
	GetTimeScale() float64
}

func GetIntensity(m MarkedHawkesProcess, t int64, params []TimedTuple) float64 {
	excitation := 0.0
	alpha := m.GetAlpha()
	beta := m.GetBeta()
	mu := m.GetMu()
	timeScale := m.GetTimeScale()

	for _, tuple := range params {
		ti := tuple.Time
		param := tuple.Params
		if ti < t {
			excitation += alpha * m.MarkFunction(param) * math.Exp(
				-beta*float64(t-ti)/timeScale)

		}
	}
	return mu + excitation
}

type VolumePriceData struct {
	Volume float64
	Price  float64
}

// Implementation of MarkedHawkesProcess for volume and price data
type MarkedHawkesProcessVP struct {
	alpha     float64
	beta      float64
	mu        float64
	timeScale float64
}

func NewMarkedHawkesProcessVP(mu, alpha, beta, timeScale float64) *MarkedHawkesProcessVP {
	return &MarkedHawkesProcessVP{
		alpha:     alpha,
		beta:      beta,
		mu:        mu,
		timeScale: timeScale,
	}
}

func (m *MarkedHawkesProcessVP) MarkFunction(params any) float64 {
	if vp, ok := params.(VolumePriceData); ok {
		// the total fund capital
		return vp.Volume * vp.Price
	}
	return 0.0
}

func (m *MarkedHawkesProcessVP) GetAlpha() float64 {
	return m.alpha
}

func (m *MarkedHawkesProcessVP) GetBeta() float64 {
	return m.beta
}

func (m *MarkedHawkesProcessVP) GetMu() float64 {
	return m.mu
}

func (m *MarkedHawkesProcessVP) GetTimeScale() float64 {
	return m.timeScale
}
