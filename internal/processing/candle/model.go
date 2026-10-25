package candle

import (
	"strconv"
)

// TimeframeCandle matches the exact requested database table schema: (symbol, o, h, l, c, v, t).
type TimeframeCandle struct {
	Symbol string `json:"symbol"`
	O      string `json:"o"`
	H      string `json:"h"`
	L      string `json:"l"`
	C      string `json:"c"`
	V      string `json:"v"`
	T      int64  `json:"t"`
}

// Compare and return the higher price as string.
func maxPrice(a, b string) string {
	fA, errA := strconv.ParseFloat(a, 64)
	fB, errB := strconv.ParseFloat(b, 64)
	if errA != nil {
		return b
	}
	if errB != nil {
		return a
	}
	if fA >= fB {
		return a
	}
	return b
}

// Compare and return the lower price as string.
func minPrice(a, b string) string {
	fA, errA := strconv.ParseFloat(a, 64)
	fB, errB := strconv.ParseFloat(b, 64)
	if errA != nil {
		return b
	}
	if errB != nil {
		return a
	}
	if fA <= fB {
		return a
	}
	return b
}

// Sum two volume strings.
func addVolume(a, b string) string {
	fA, _ := strconv.ParseFloat(a, 64)
	fB, _ := strconv.ParseFloat(b, 64)
	sum := fA + fB
	return strconv.FormatFloat(sum, 'f', -1, 64)
}
