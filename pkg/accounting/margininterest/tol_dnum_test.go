//go:build dnum

package margininterest

// dnum keeps 16 significant digits; Float64 conversion still rounds
const tol = 1e-12
