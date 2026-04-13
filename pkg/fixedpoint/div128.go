//go:build dnum

// Copyright Suneido Software Corp. All rights reserved.
// Governed by the MIT license found in the LICENSE file.

package fixedpoint

import (
	"math/bits"
)

const (
	e16        = 1_0000_0000_0000_0000
	longMask   = 0xffffffff
	divNumBase = 1 << 32
	e16Hi      = e16 >> 32
	e16Lo      = e16 & longMask
)

// returns (1e16 * dividend) / divisor
// Used by dnum divide
// Based on cSuneido code
// which is based on jSuneido code
// which is based on Java BigDecimal code
// which is based on Hacker's Delight and Knuth TAoCP Vol 2
// A bit simpler with unsigned types
func div128(dividend, divisor uint64) uint64 {
	//check(dividend != 0)
	//check(divisor != 0)
	// multiply dividend * e16
	d1Hi := dividend >> 32
	d1Lo := dividend & longMask
	product := uint64(e16Lo) * d1Lo
	d0 := product & longMask
	d1 := product >> 32
	product = uint64(e16Hi)*d1Lo + d1
	d1 = product & longMask
	d2 := product >> 32
	product = uint64(e16Lo)*d1Hi + d1
	d1 = product & longMask
	d2 += product >> 32
	d3 := d2 >> 32
	d2 &= longMask
	product = e16Hi*d1Hi + d2
	d2 = product & longMask
	d3 = ((product >> 32) + d3) & longMask
	dividendHi := make64(uint32(d3), uint32(d2))
	dividendLo := make64(uint32(d1), uint32(d0))
	// divide
	return divide128(dividendHi, dividendLo, divisor)
}

func divide128(dividendHi, dividendLo, divisor uint64) uint64 {
	// so we can shift dividend as much as divisor
	// don't allow equals to avoid quotient overflow (by 1)
	//check(dividendHi < divisor)

	// maximize divisor (bit wise), since we're mostly using the top half
	shift := uint(bits.LeadingZeros64(divisor))
	divisor = divisor << shift

	// split divisor
	v1 := divisor >> 32
	v0 := divisor & longMask

	// matching shift
	dls := dividendLo << shift
	// split dividendLo
	u1 := uint32(dls >> 32)
	u0 := uint32(dls & longMask)

	// tmp1 = top 64 of dividend << shift
	tmp1 := (dividendHi << shift) | (dividendLo >> (64 - shift))
	var q1, rtmp1 uint64
	if v1 == 1 {
		q1 = tmp1
		rtmp1 = 0
	} else {
		//check(tmp1 >= 0)
		q1 = tmp1 / v1    // DIVIDE top 64 / top 32
		rtmp1 = tmp1 % v1 // remainder
	}

	// adjust if quotient estimate too large
	//check(q1 < divNumBase)
	for q1*v0 > make64(uint32(rtmp1), u1) {
		// done about 5.5 per 10,000 divides
		q1--
		rtmp1 += v1
		if rtmp1 >= divNumBase {
			break
		}
	}
	//check(q1 >= 0)
	u2 := tmp1 & longMask // low half

	// u2,u1 is the MIDDLE 64 bits of the dividend
	tmp2 := mulsub(uint32(u2), uint32(u1), uint32(v1), uint32(v0), q1)
	var q0, rtmp2 uint64
	if v1 == 1 {
		q0 = tmp2
		rtmp2 = 0
	} else {
		q0 = tmp2 / v1 // DIVIDE dividend remainder 64 / divisor high 32
		rtmp2 = tmp2 % v1
	}

	// adjust if quotient estimate too large
	//check(q0 < divNumBase)
	for q0*v0 > make64(uint32(rtmp2), u0) {
		// done about .33 times per divide
		q0--
		rtmp2 += v1
		if rtmp2 >= divNumBase {
			break
		}
		//check(q0 < divNumBase)
	}

	//check(q1 <= math.MaxUint32)
	//check(q0 <= math.MaxUint32)
	return make64(uint32(q1), uint32(q0))
}

// mulsub returns u1,u0 - v1,v0 * q0
func mulsub(u1, u0, v1, v0 uint32, q0 uint64) uint64 {
	tmp := uint64(u0) - q0*uint64(v0)
	return make64(u1+uint32(tmp>>32)-uint32(q0*uint64(v1)), uint32(tmp&longMask))
}

func make64(hi, lo uint32) uint64 {
	return uint64(hi)<<32 | uint64(lo)
}
