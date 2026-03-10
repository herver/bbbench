package main

import (
	"math"
	"testing"
)

// TestParseFioSingleLog_IOPS verifies that IOPS values from 10 job files are
// correctly aggregated: timestamps in the 30050–30208ms range all map to the
// 1-second bucket sec=30, and their values are summed (not averaged).
func TestParseFioSingleLog_IOPS(t *testing.T) {
	prefix := "testdata/fio_logs/sdg_seq_write_log"
	pts := parseFioSingleLog(prefix, "iops", 1.0, false)
	if len(pts) == 0 {
		t.Fatal("expected time points, got none")
	}

	// Find the point at t=30 (the first 30-second averaging window).
	var found bool
	for _, p := range pts {
		if p.T != 30 {
			continue
		}
		found = true
		// All entries are write (direction=1); read should be zero.
		if p.R != 0 {
			t.Errorf("IOPS t=30: R = %.2f, want 0", p.R)
		}
		// Sum of 10 files × 39 IOPS each = 390 exactly.
		wantW := 390.0
		if math.Abs(p.W-wantW) > 1.0 {
			t.Errorf("IOPS t=30: W = %.2f, want %.2f (±1)", p.W, wantW)
		}
	}
	if !found {
		t.Error("no time point found at t=30")
	}
}

// TestParseFioSingleLog_BW verifies bandwidth aggregation: KiB/s values from
// 10 job files are summed and converted to MB/s via scale=1/1024.
func TestParseFioSingleLog_BW(t *testing.T) {
	prefix := "testdata/fio_logs/sdg_seq_write_log"
	pts := parseFioSingleLog(prefix, "bw", 1.0/1024.0, false)
	if len(pts) == 0 {
		t.Fatal("expected time points, got none")
	}

	var found bool
	for _, p := range pts {
		if p.T != 30 {
			continue
		}
		found = true
		if p.R != 0 {
			t.Errorf("BW t=30: R = %.4f, want 0", p.R)
		}
		// Sum of 10 files = 1575 KiB/s; scaled → 1575/1024 ≈ 1.5381 MB/s.
		wantW := 1575.0 / 1024.0
		if math.Abs(p.W-wantW) > 0.01 {
			t.Errorf("BW t=30: W = %.4f MB/s, want %.4f MB/s (±0.01)", p.W, wantW)
		}
	}
	if !found {
		t.Error("no time point found at t=30")
	}
}

// TestParseFioSingleLog_Clat verifies completion-latency aggregation: ns values
// from 10 job files are averaged (not summed) and converted to µs via
// scale=1/1000.
func TestParseFioSingleLog_Clat(t *testing.T) {
	prefix := "testdata/fio_logs/sdg_seq_write_log"
	// The test files are named _clat.*.log; use "clat" as the kind.
	pts := parseFioSingleLog(prefix, "clat", 1.0/1000.0, true)
	if len(pts) == 0 {
		t.Fatal("expected time points, got none")
	}

	var found bool
	for _, p := range pts {
		if p.T != 30 {
			continue
		}
		found = true
		if p.R != 0 {
			t.Errorf("Clat t=30: R = %.2f, want 0", p.R)
		}
		// Average of 10 files' clat values at sec=30, then scaled ns → µs.
		// Sum from actual log files = 32,023,445,176 ns; avg = 3,202,344,517.6 ns.
		// Scaled: 3,202,344,517.6 / 1000 ≈ 3,202,344.5 µs.
		wantW := 32023445176.0 / 10.0 / 1000.0
		if math.Abs(p.W-wantW) > 1000 {
			t.Errorf("Clat t=30: W = %.2f µs, want %.2f µs (±1000)", p.W, wantW)
		}
	}
	if !found {
		t.Error("no time point found at t=30")
	}
}

// TestParseFioSingleLog_TimestampBucketing verifies that timestamp jitter
// between job files (e.g. 30050ms vs 30208ms) does not produce separate
// time points — all entries for the same 30-second window must merge into
// a single point at t=30.
func TestParseFioSingleLog_TimestampBucketing(t *testing.T) {
	prefix := "testdata/fio_logs/sdg_seq_write_log"
	pts := parseFioSingleLog(prefix, "iops", 1.0, false)
	if len(pts) == 0 {
		t.Fatal("expected time points, got none")
	}

	// Count how many points fall in the [30, 31) second range.
	count := 0
	for _, p := range pts {
		if p.T >= 30 && p.T < 31 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 point in [30,31)s range, got %d — "+
			"timestamp jitter between job files is not being bucketed correctly", count)
	}
}

// TestParseFioSingleLog_AllPointsPresent verifies that time points are present
// and sorted. Each file has 19 entries (~30s to ~570s in 30s steps). Small
// timestamp jitter (e.g. 540894ms vs 541006ms) may split one window across two
// adjacent 1-second buckets, so the actual count can be slightly above 19.
func TestParseFioSingleLog_AllPointsPresent(t *testing.T) {
	prefix := "testdata/fio_logs/sdg_seq_write_log"
	pts := parseFioSingleLog(prefix, "iops", 1.0, false)

	// At least 19 distinct time points must be present.
	if len(pts) < 19 {
		t.Errorf("expected at least 19 time points, got %d", len(pts))
	}

	// Verify points are sorted in ascending order.
	for i := 1; i < len(pts); i++ {
		if pts[i].T <= pts[i-1].T {
			t.Errorf("points not sorted: pts[%d].T=%.0f <= pts[%d].T=%.0f",
				i, pts[i].T, i-1, pts[i-1].T)
		}
	}
}

// TestParseFioLogFiles verifies the combined parsing of all three metric types
// through the higher-level parseFioLogFiles function. It uses "clat" files
// which are present in the testdata, but parseFioLogFiles looks for "lat" — so
// only iops and bw should be populated from these testdata files.
func TestParseFioLogFiles_IOPSAndBW(t *testing.T) {
	prefix := "testdata/fio_logs/sdg_seq_write_log"
	ts := parseFioLogFiles(prefix)
	if ts == nil {
		t.Fatal("parseFioLogFiles returned nil")
	}

	if len(ts.IOPS) == 0 {
		t.Error("expected IOPS time series, got none")
	}
	if len(ts.BW) == 0 {
		t.Error("expected BW time series, got none")
	}
	// Lat uses "lat" suffix; testdata only has "clat" files, so Lat will be empty.
	if len(ts.Lat) != 0 {
		t.Errorf("expected no Lat series (testdata uses 'clat' not 'lat'), got %d points", len(ts.Lat))
	}

	// Spot-check IOPS at t=30.
	var gotIOPS bool
	for _, p := range ts.IOPS {
		if p.T == 30 {
			gotIOPS = true
			if math.Abs(p.W-390) > 1 {
				t.Errorf("IOPS via parseFioLogFiles at t=30: W=%.2f, want 390", p.W)
			}
		}
	}
	if !gotIOPS {
		t.Error("no IOPS point at t=30")
	}
}
