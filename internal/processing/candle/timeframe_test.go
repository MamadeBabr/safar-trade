package candle

import (
	"testing"
	"time"
)

func TestCalculateBucketTime(t *testing.T) {
	// 2026-10-25 20:37:45 UTC = 1792960665000 ms (Sunday)
	baseTime := time.Date(2026, 10, 25, 20, 37, 45, 0, time.UTC).UnixMilli()

	// 5m: 20:35:00
	expected5m := time.Date(2026, 10, 25, 20, 35, 0, 0, time.UTC).UnixMilli()
	if got := CalculateBucketTime(baseTime, TF5m); got != expected5m {
		t.Errorf("5m: expected %d (%v), got %d (%v)", expected5m, time.UnixMilli(expected5m), got, time.UnixMilli(got))
	}

	// 15m: 20:30:00
	expected15m := time.Date(2026, 10, 25, 20, 30, 0, 0, time.UTC).UnixMilli()
	if got := CalculateBucketTime(baseTime, TF15m); got != expected15m {
		t.Errorf("15m: expected %d, got %d", expected15m, got)
	}

	// 30m: 20:30:00
	expected30m := time.Date(2026, 10, 25, 20, 30, 0, 0, time.UTC).UnixMilli()
	if got := CalculateBucketTime(baseTime, TF30m); got != expected30m {
		t.Errorf("30m: expected %d, got %d", expected30m, got)
	}

	// 60m: 20:00:00
	expected60m := time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC).UnixMilli()
	if got := CalculateBucketTime(baseTime, TF60m); got != expected60m {
		t.Errorf("60m: expected %d, got %d", expected60m, got)
	}

	// 240m: 20:00:00 (since 20 is a multiple of 4: 0, 4, 8, 12, 16, 20)
	expected240m := time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC).UnixMilli()
	if got := CalculateBucketTime(baseTime, TF240m); got != expected240m {
		t.Errorf("240m: expected %d, got %d", expected240m, got)
	}

	// 1d: 2026-10-25 00:00:00
	expected1d := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC).UnixMilli()
	if got := CalculateBucketTime(baseTime, TF1d); got != expected1d {
		t.Errorf("1d: expected %d, got %d", expected1d, got)
	}

	// 1w: Sunday 2026-10-25 belongs to week starting Monday 2026-10-19 00:00:00
	expected1w := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC).UnixMilli()
	if got := CalculateBucketTime(baseTime, TF1w); got != expected1w {
		t.Errorf("1w: expected %d (%v), got %d (%v)", expected1w, time.UnixMilli(expected1w), got, time.UnixMilli(got))
	}

	// 1M: 2026-10-01 00:00:00
	expected1M := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if got := CalculateBucketTime(baseTime, TF1M); got != expected1M {
		t.Errorf("1M: expected %d, got %d", expected1M, got)
	}
}

func BenchmarkCalculateBucketTime(b *testing.B) {
	t := time.Now().UnixMilli()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateBucketTime(t, TF5m)
		_ = CalculateBucketTime(t, TF15m)
		_ = CalculateBucketTime(t, TF30m)
		_ = CalculateBucketTime(t, TF60m)
		_ = CalculateBucketTime(t, TF240m)
		_ = CalculateBucketTime(t, TF1d)
		_ = CalculateBucketTime(t, TF1w)
		_ = CalculateBucketTime(t, TF1M)
	}
}
