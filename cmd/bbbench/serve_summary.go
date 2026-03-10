package main

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strings"
)

// StatSummary holds min/max/avg/stddev for one metric across disks.
type StatSummary struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	StdDev float64 `json:"stddev"`
	Count  int     `json:"count"`
}

// TestSummaryRow is one row in the summary table (one test/phase name).
type TestSummaryRow struct {
	TestName    string       `json:"test_name"`
	DiskCount   int          `json:"disk_count"`
	ReadIOPS    *StatSummary `json:"read_iops,omitempty"`
	WriteIOPS   *StatSummary `json:"write_iops,omitempty"`
	ReadBWMbps  *StatSummary `json:"read_bw_mbps,omitempty"`
	WriteBWMbps *StatSummary `json:"write_bw_mbps,omitempty"`
	ReadLatUS   *StatSummary `json:"read_lat_us,omitempty"`
	WriteLatUS  *StatSummary `json:"write_lat_us,omitempty"`
}

func statSummary(vals []float64) *StatSummary {
	if len(vals) == 0 {
		return nil
	}
	mn, mx := vals[0], vals[0]
	var sum float64
	for _, v := range vals {
		sum += v
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	avg := sum / float64(len(vals))
	var variance float64
	for _, v := range vals {
		d := v - avg
		variance += d * d
	}
	variance /= float64(len(vals))
	return &StatSummary{
		Min:    mn,
		Max:    mx,
		Avg:    avg,
		StdDev: math.Sqrt(variance),
		Count:  len(vals),
	}
}

// computeSummary aggregates DiskPhaseData into per-test statistics across disks.
func computeSummary(result *BenchmarkResult) []TestSummaryRow {
	flat := generateDiskPhaseData(result)

	// Collect values per test name, preserving insertion order.
	type testAccum struct {
		disks     map[string]bool
		readIOPS  []float64
		writeIOPS []float64
		readBW    []float64
		writeBW   []float64
		readLat   []float64
		writeLat  []float64
	}
	order := []string{}
	accum := map[string]*testAccum{}

	for _, dp := range flat {
		name := dp.PhaseName
		if _, ok := accum[name]; !ok {
			order = append(order, name)
			accum[name] = &testAccum{disks: map[string]bool{}}
		}
		a := accum[name]
		a.disks[dp.Disk] = true
		if dp.HasRead {
			a.readIOPS = append(a.readIOPS, dp.ReadIOPS)
			a.readBW = append(a.readBW, dp.ReadBWMbps)
			if dp.ReadLatUS > 0 {
				a.readLat = append(a.readLat, dp.ReadLatUS)
			}
		}
		if dp.HasWrite {
			a.writeIOPS = append(a.writeIOPS, dp.WriteIOPS)
			a.writeBW = append(a.writeBW, dp.WriteBWMbps)
			if dp.WriteLatUS > 0 {
				a.writeLat = append(a.writeLat, dp.WriteLatUS)
			}
		}
	}

	rows := make([]TestSummaryRow, 0, len(order))
	for _, name := range order {
		a := accum[name]
		rows = append(rows, TestSummaryRow{
			TestName:    name,
			DiskCount:   len(a.disks),
			ReadIOPS:    statSummary(a.readIOPS),
			WriteIOPS:   statSummary(a.writeIOPS),
			ReadBWMbps:  statSummary(a.readBW),
			WriteBWMbps: statSummary(a.writeBW),
			ReadLatUS:   statSummary(a.readLat),
			WriteLatUS:  statSummary(a.writeLat),
		})
	}
	return rows
}

// handleResultSummaryAPI serves the summary data as JSON.
func handleResultSummaryAPI(w http.ResponseWriter, r *http.Request) {
	resultID := r.URL.Query().Get("id")
	if resultID == "" {
		http.Error(w, "Missing result ID", http.StatusBadRequest)
		return
	}
	result, ok := globalResultsStore.GetResult(resultID)
	if !ok {
		http.Error(w, "Result not found", http.StatusNotFound)
		return
	}
	rows := computeSummary(result)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rows); err != nil {
		if logger != nil {
			logger.Error("encode summary", "err", err)
		}
	}
}

// handleSummaryPage serves the summary statistics HTML page.
func handleSummaryPage(w http.ResponseWriter, r *http.Request) {
	resultID := r.URL.Query().Get("id")
	if resultID == "" {
		http.Error(w, "Missing result ID", http.StatusBadRequest)
		return
	}
	result, ok := globalResultsStore.GetResult(resultID)
	if !ok {
		http.Error(w, "Result not found", http.StatusNotFound)
		return
	}

	// Collect drive names for the subtitle, sorted.
	driveNames := make([]string, 0, len(result.Drives))
	seen := map[string]bool{}
	for _, d := range result.Drives {
		if !seen[d.Device.Name] {
			seen[d.Device.Name] = true
			driveNames = append(driveNames, d.Device.Name)
		}
	}
	sort.Strings(driveNames)
	subtitle := strings.Join(driveNames, ", ")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	html := `<!DOCTYPE html>
<html>
<head>
    <title>bbbench - Summary</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: #f1f5f9;
        }
        __NAVBAR_CSS__
        .container { max-width: 1400px; margin: 0 auto; padding: 24px 20px; }
        .page-title {
            font-size: 20px; font-weight: 700; color: #1e293b;
            margin-bottom: 4px;
        }
        .page-sub {
            font-size: 13px; color: #64748b; margin-bottom: 20px;
        }
        .card {
            background: #fff; border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
            overflow: hidden; margin-bottom: 28px;
        }
        .card-header {
            padding: 14px 18px; border-bottom: 1px solid #e2e8f0;
            font-size: 14px; font-weight: 700; color: #1e293b;
            display: flex; align-items: center; gap: 10px;
        }
        .disk-badge {
            font-size: 11px; font-weight: 600; color: #64748b;
            background: #f1f5f9; border: 1px solid #e2e8f0;
            border-radius: 4px; padding: 2px 8px;
        }
        table {
            width: 100%; border-collapse: collapse;
        }
        thead th {
            background: #f8fafc;
            padding: 8px 12px;
            font-size: 11px; font-weight: 700; color: #64748b;
            text-transform: uppercase; letter-spacing: 0.06em;
            border-bottom: 2px solid #e2e8f0;
            text-align: right;
            white-space: nowrap;
        }
        thead th.test-col {
            text-align: left; min-width: 200px;
        }
        thead th.group-sep {
            border-left: 2px solid #e2e8f0;
        }
        tbody td {
            padding: 9px 12px;
            font-size: 13px; color: #334155;
            border-bottom: 1px solid #f1f5f9;
            text-align: right;
            white-space: nowrap;
        }
        tbody td.test-col {
            text-align: left; font-weight: 600; color: #1e293b;
        }
        tbody td.group-sep {
            border-left: 2px solid #e2e8f0;
        }
        tbody tr:last-child td { border-bottom: none; }
        tbody tr:hover td { background: #f8fafc; }
        .na { color: #cbd5e1; }
        .subhdr th {
            background: #fff; padding: 4px 12px;
            font-size: 10px; color: #94a3b8;
            text-transform: uppercase; font-weight: 600;
            border-bottom: 1px solid #e2e8f0;
        }
        .subhdr th.group-sep { border-left: 2px solid #e2e8f0; }
        .placeholder {
            padding: 48px; text-align: center;
            color: #94a3b8; font-size: 15px;
        }
        .metric-group-label {
            text-align: center !important;
            background: #f8fafc !important;
        }
    </style>
</head>
<body>
__NAVBAR_HTML__
<div class="container">
    <div class="page-title">Benchmark Summary</div>
    <div class="page-sub">Result ID: __RESULT_ID__ &nbsp;·&nbsp; Drives: __SUBTITLE__</div>

    <div id="iops-card" class="card">
        <div class="card-header">IOPS</div>
        <div id="iops-table"><div class="placeholder">Loading…</div></div>
    </div>
    <div id="bw-card" class="card">
        <div class="card-header">Bandwidth (MB/s)</div>
        <div id="bw-table"><div class="placeholder">Loading…</div></div>
    </div>
    <div id="lat-card" class="card">
        <div class="card-header">Latency (µs)</div>
        <div id="lat-table"><div class="placeholder">Loading…</div></div>
    </div>
</div>

<script>
const resultID = '__RESULT_ID__';

function fmt(v, decimals) {
    if (v == null) return '<span class="na">—</span>';
    if (v >= 1e6)  return (v/1e6).toFixed(1) + 'M';
    if (v >= 1e3)  return (v/1e3).toFixed(1) + 'k';
    return v.toFixed(decimals != null ? decimals : 1);
}

function fmtLat(v) {
    if (v == null) return '<span class="na">—</span>';
    if (v >= 1e6)  return (v/1e6).toFixed(2) + 's';
    if (v >= 1e3)  return (v/1e3).toFixed(1) + 'ms';
    return v.toFixed(1) + 'µs';
}

function buildTable(rows, rVal, wVal, fmtFn, diskCounts) {
    const hasRead  = rows.some(r => rVal(r) != null);
    const hasWrite = rows.some(r => wVal(r) != null);

    if (!hasRead && !hasWrite) {
        return '<div class="placeholder">No data for this metric.</div>';
    }

    let colsR = hasRead  ? ['Min','Avg','Max','StdDev'] : [];
    let colsW = hasWrite ? ['Min','Avg','Max','StdDev'] : [];

    let h1 = '<thead><tr><th class="test-col" rowspan="2">Test</th>';
    if (hasRead)  h1 += '<th class="group-sep metric-group-label" colspan="4">Read</th>';
    if (hasWrite) h1 += '<th class="group-sep metric-group-label" colspan="4">Write</th>';
    h1 += '</tr>';

    let h2 = '<tr class="subhdr">';
    if (hasRead)  h2 += '<th class="group-sep">Min</th><th>Avg</th><th>Max</th><th>StdDev</th>';
    if (hasWrite) h2 += '<th class="group-sep">Min</th><th>Avg</th><th>Max</th><th>StdDev</th>';
    h2 += '</tr></thead>';

    let body = '<tbody>';
    rows.forEach(r => {
        const rv = rVal(r), wv = wVal(r);
        let row = '<tr>';
        const diskLabel = (r.disk_count > 0)
            ? ' <span class="disk-badge">n=' + r.disk_count + '</span>'
            : '';
        row += '<td class="test-col">' + r.test_name + diskLabel + '</td>';
        if (hasRead) {
            if (rv) {
                row += '<td class="group-sep">' + fmtFn(rv.min)    + '</td>'
                     + '<td>'                   + fmtFn(rv.avg)    + '</td>'
                     + '<td>'                   + fmtFn(rv.max)    + '</td>'
                     + '<td>'                   + fmtFn(rv.stddev) + '</td>';
            } else {
                row += '<td class="group-sep na" colspan="4">—</td>';
            }
        }
        if (hasWrite) {
            if (wv) {
                row += '<td class="group-sep">' + fmtFn(wv.min)    + '</td>'
                     + '<td>'                   + fmtFn(wv.avg)    + '</td>'
                     + '<td>'                   + fmtFn(wv.max)    + '</td>'
                     + '<td>'                   + fmtFn(wv.stddev) + '</td>';
            } else {
                row += '<td class="group-sep na" colspan="4">—</td>';
            }
        }
        row += '</tr>';
        body += row;
    });
    body += '</tbody>';

    return '<table>' + h1 + h2 + body + '</table>';
}

fetch('/api/results/summary?id=' + resultID)
    .then(r => r.json())
    .then(rows => {
        if (!rows || rows.length === 0) {
            const msg = '<div class="placeholder">No summary data available.</div>';
            document.getElementById('iops-table').innerHTML = msg;
            document.getElementById('bw-table').innerHTML  = msg;
            document.getElementById('lat-table').innerHTML  = msg;
            return;
        }

        document.getElementById('iops-table').innerHTML = buildTable(rows,
            r => r.read_iops,   r => r.write_iops,
            v => fmt(v, 0), null);

        document.getElementById('bw-table').innerHTML = buildTable(rows,
            r => r.read_bw_mbps, r => r.write_bw_mbps,
            v => fmt(v, 1), null);

        document.getElementById('lat-table').innerHTML = buildTable(rows,
            r => r.read_lat_us,  r => r.write_lat_us,
            fmtLat, null);
    })
    .catch(err => {
        const msg = '<div class="placeholder">Error loading data: ' + err.message + '</div>';
        document.getElementById('iops-table').innerHTML = msg;
        document.getElementById('bw-table').innerHTML  = msg;
        document.getElementById('lat-table').innerHTML  = msg;
    });
</script>
</body>
</html>`

	html = strings.ReplaceAll(html, "__NAVBAR_CSS__", navbarCSS)
	html = strings.ReplaceAll(html, "__NAVBAR_HTML__", navbarHTML("/results"))
	html = strings.ReplaceAll(html, "__RESULT_ID__", escapeHTML(resultID))
	html = strings.ReplaceAll(html, "__SUBTITLE__", escapeHTML(subtitle))

	w.Write([]byte(html))
}
