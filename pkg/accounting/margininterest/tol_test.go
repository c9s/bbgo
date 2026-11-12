//go:build !dnum

package margininterest

// non-dnum Mul truncates every product to 8 decimals
const tol = 1e-7
