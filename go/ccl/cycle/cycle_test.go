package cycle

import (
	"testing"
	"time"
)

// resetClock is the default reset clock for most tests: 2:59 PM.
var defaultClock = Clock{Hour: 14, Minute: 59}

// mSa is Monday through Saturday (6 days).
var mSa = []time.Weekday{
	time.Monday, time.Tuesday, time.Wednesday,
	time.Thursday, time.Friday, time.Saturday,
}

// mF is Monday through Friday (5 days).
var mF = []time.Weekday{
	time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
}

// allDays is all 7 days.
var allDays = []time.Weekday{
	time.Sunday, time.Monday, time.Tuesday, time.Wednesday,
	time.Thursday, time.Friday, time.Saturday,
}

// date is a convenience helper to build a local time.
func date(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.Local)
}

// approx asserts two floats are within epsilon of each other.
func approx(t *testing.T, got, want, epsilon float64, label string) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > epsilon {
		t.Errorf("%s: got %.6f, want %.6f (±%.6f)", label, got, want, epsilon)
	}
}

// --- FindCycleStart tests ---

func TestFindCycleStart(t *testing.T) {
	// All tests use Thu 2:59 PM as the reset.
	resetWD := time.Thursday
	clk := defaultClock

	tests := []struct {
		label string
		now   time.Time
		want  time.Time
	}{
		{
			label: "now is exactly at reset on reset day -> this cycle",
			now:   date(2024, time.April, 18, 14, 59), // Thu
			want:  date(2024, time.April, 18, 14, 59),
		},
		{
			label: "now is after reset on reset day -> this cycle",
			now:   date(2024, time.April, 18, 16, 0), // Thu, after 2:59 PM
			want:  date(2024, time.April, 18, 14, 59),
		},
		{
			label: "now is before reset on reset day -> previous week",
			now:   date(2024, time.April, 18, 10, 0), // Thu, before 2:59 PM
			want:  date(2024, time.April, 11, 14, 59), // prior Thursday
		},
		{
			label: "now is mid-week (Monday) -> previous Thursday",
			now:   date(2024, time.April, 22, 9, 0), // Mon
			want:  date(2024, time.April, 18, 14, 59),
		},
		{
			label: "now is just before cycle end",
			now:   date(2024, time.April, 25, 14, 58), // next Thu, 1 min before
			want:  date(2024, time.April, 18, 14, 59),
		},
		{
			label: "now is exactly at new cycle start",
			now:   date(2024, time.April, 25, 14, 59), // next Thu, exactly
			want:  date(2024, time.April, 25, 14, 59),
		},
		{
			label: "now is Sunday",
			now:   date(2024, time.April, 21, 12, 0), // Sun
			want:  date(2024, time.April, 18, 14, 59),
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			got := FindCycleStart(tc.now, resetWD, clk)
			if !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// --- ExpectedUsage tests ---

// Segments are end-based: segment [day_i, day_i+1) is tagged by weekday of day_i+1.
// For reset=Thu 2:59 PM, work=M-Sa (6 days), per_day = 16.6̄%:
//
//	Seg 0 [Thu 14:59, Fri 14:59) -> Fri (work)
//	Seg 1 [Fri 14:59, Sat 14:59) -> Sat (work)
//	Seg 2 [Sat 14:59, Sun 14:59) -> Sun (NON-work)
//	Seg 3 [Sun 14:59, Mon 14:59) -> Mon (work)
//	Seg 4 [Mon 14:59, Tue 14:59) -> Tue (work)
//	Seg 5 [Tue 14:59, Wed 14:59) -> Wed (work)
//	Seg 6 [Wed 14:59, Thu 14:59) -> Thu (work)
func TestExpectedUsage(t *testing.T) {
	const perDay6 = 100.0 / 6.0 // ≈ 16.6̄%

	tests := []struct {
		label    string
		now      time.Time
		workDays []time.Weekday
		resetWD  time.Weekday
		clock    Clock
		want     float64
		epsilon  float64
	}{
		{
			label:    "cycle start exactly -> 0%",
			now:      date(2024, time.April, 18, 14, 59), // Thu 2:59 PM
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     0.0,
			epsilon:  0.001,
		},
		{
			label:    "end of first work-day segment (Fri 2:59 PM) -> 1 day done",
			now:      date(2024, time.April, 19, 14, 59), // Fri 2:59 PM = start of seg 1
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     perDay6, // 16.6̄%
			epsilon:  0.001,
		},
		{
			label:    "midpoint of first work-day segment (Fri 2:59 AM) -> 0.5 days done",
			now:      date(2024, time.April, 19, 2, 59), // Fri 2:59 AM, 12h into seg 0
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     perDay6 / 2, // 8.33̄%
			epsilon:  0.001,
		},
		{
			label:    "start of Sat segment -> 2 work days done",
			now:      date(2024, time.April, 20, 14, 59), // Sat 2:59 PM = start of seg 2 (Sun non-work)
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     2 * perDay6, // 33.3̄%
			epsilon:  0.001,
		},
		{
			label:    "non-work day Sunday noon -> flat at 2 days done",
			now:      date(2024, time.April, 21, 12, 0), // Sun 12:00 PM (in Sun non-work seg)
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     2 * perDay6, // 33.3̄%
			epsilon:  0.001,
		},
		{
			label:    "non-work day boundary Sun 2:59 PM -> start of Mon seg, 2 days done",
			now:      date(2024, time.April, 21, 14, 59), // Sun 2:59 PM = start of Mon seg
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     2 * perDay6, // 33.3̄%
			epsilon:  0.001,
		},
		{
			label:    "start of Mon segment -> 3 work days done (Fri+Sat done, Sun off, Mon starting)",
			now:      date(2024, time.April, 22, 14, 59), // Mon 2:59 PM
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     3 * perDay6, // 50%
			epsilon:  0.001,
		},
		{
			label:    "just before cycle end (Thu 2:58 PM next week) -> ~99.99%",
			now:      date(2024, time.April, 25, 14, 58), // Thu 2:58 PM
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     (5.0 + float64(24*60-1)/float64(24*60)) * perDay6,
			epsilon:  0.01,
		},
		{
			label:    "exactly at cycle end (new cycle) -> 0%",
			now:      date(2024, time.April, 25, 14, 59), // Thu 2:59 PM next week
			workDays: mSa,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     0.0,
			epsilon:  0.001,
		},
		{
			label:    "only 1 work day (Fri), at Fri 2:59 PM -> 100%",
			now:      date(2024, time.April, 19, 14, 59), // Fri 2:59 PM
			workDays: []time.Weekday{time.Friday},
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     100.0,
			epsilon:  0.001,
		},
		{
			label:    "only 1 work day (Fri), mid-segment -> 50%",
			now:      date(2024, time.April, 19, 2, 59), // Fri 2:59 AM, 12h into seg
			workDays: []time.Weekday{time.Friday},
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     50.0,
			epsilon:  0.001,
		},
		{
			label:    "all 7 days work, Wed 3:00 PM -> start of final work seg",
			now:      date(2024, time.April, 24, 15, 0), // Wed 3:00 PM, just past 2:59 PM
			workDays: allDays,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     (6.0 + float64(1)/float64(24*60)) * (100.0 / 7.0),
			epsilon:  0.05,
		},
		{
			label:    "all 7 days work, Thu 2:58 PM -> nearly 100%",
			now:      date(2024, time.April, 25, 14, 58), // Thu 2:58 PM
			workDays: allDays,
			resetWD:  time.Thursday,
			clock:    defaultClock,
			want:     (6.0 + float64(24*60-1)/float64(24*60)) * (100.0 / 7.0),
			epsilon:  0.05,
		},
		{
			label:    "5-day M-F, Mon reset, at Mon 9 AM (start) -> 0%",
			now:      date(2024, time.April, 22, 9, 0), // Mon 9:00 AM
			workDays: mF,
			resetWD:  time.Monday,
			clock:    Clock{Hour: 9, Minute: 0},
			want:     0.0,
			epsilon:  0.001,
		},
		{
			label:    "5-day M-F, Mon reset, at Wed 9 AM -> 2 days done",
			now:      date(2024, time.April, 24, 9, 0), // Wed 9:00 AM
			workDays: mF,
			resetWD:  time.Monday,
			clock:    Clock{Hour: 9, Minute: 0},
			want:     2 * 20.0, // 40%
			epsilon:  0.001,
		},
		{
			label:    "5-day M-F, Mon reset, Sat noon -> flat (4 work days done before weekend)",
			now:      date(2024, time.April, 20, 12, 0), // Sat noon
			workDays: mF,
			resetWD:  time.Monday,
			clock:    Clock{Hour: 9, Minute: 0},
			want:     4 * 20.0, // 80% -- Fri seg tags Sat (non-work), so it's flat after Fri seg done
			epsilon:  0.001,
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			got := ExpectedUsage(tc.now, tc.workDays, tc.resetWD, tc.clock)
			approx(t, got, tc.want, tc.epsilon, "ExpectedUsage")
		})
	}
}

// TestExpectedUsageMonotonicity verifies that the expected usage is monotonically
// non-decreasing across a full 7-day cycle, stays within [0, 100], is flat on
// non-work days, and reaches ~100% just before the cycle ends.
func TestExpectedUsageMonotonicity(t *testing.T) {
	cycleStart := date(2024, time.April, 18, 14, 59) // Thu 2:59 PM
	resetWD := time.Thursday
	clk := defaultClock
	workDays := mSa

	step := time.Minute
	prev := -1.0
	var lastVal float64

	for d := time.Duration(0); d < 7*24*time.Hour; d += step {
		now := cycleStart.Add(d)
		v := ExpectedUsage(now, workDays, resetWD, clk)

		if v < 0 || v > 100 {
			t.Errorf("at %v: value %.4f out of [0,100]", now, v)
		}
		if prev >= 0 && v < prev-0.001 {
			t.Errorf("at %v: value decreased from %.6f to %.6f", now, prev, v)
		}
		prev = v
		lastVal = v
	}

	// Just before cycle end, should be near 100%.
	if lastVal < 99.5 {
		t.Errorf("expected near 100%% at cycle end, got %.4f", lastVal)
	}
}

// TestExpectedUsageFlatOnNonWorkDays verifies that usage is constant during
// each non-work day segment.
func TestExpectedUsageFlatOnNonWorkDays(t *testing.T) {
	// Sun is the only non-work day.  Seg 2 [Sat 14:59, Sun 14:59) -> Sunday non-work.
	cycleStart := date(2024, time.April, 18, 14, 59)
	resetWD := time.Thursday
	clk := defaultClock
	workDays := mSa

	// Sample 60 points inside the Sunday non-work segment.
	sunSegStart := cycleStart.Add(2 * 24 * time.Hour) // Sat 14:59
	refVal := ExpectedUsage(sunSegStart, workDays, resetWD, clk)
	for m := time.Duration(0); m < 24*time.Hour; m += 15 * time.Minute {
		now := sunSegStart.Add(m)
		v := ExpectedUsage(now, workDays, resetWD, clk)
		approx(t, v, refVal, 0.001, "non-work day should be flat")
	}
}

// TestParseResetTime validates parsing of various reset time strings.
func TestParseResetTime(t *testing.T) {
	tests := []struct {
		input      string
		wantWD     time.Weekday
		wantHour   int
		wantMinute int
		wantErr    bool
	}{
		{"Thu 2:59 PM", time.Thursday, 14, 59, false},
		{"Mon 11:00 AM", time.Monday, 11, 0, false},
		{"Fri 12:00 PM", time.Friday, 12, 0, false},
		{"Sat 9:00 AM", time.Saturday, 9, 0, false},
		{"Sun 11:59 PM", time.Sunday, 23, 59, false},
		{"", 0, 0, 0, true},
		{"BadDay 2:00 PM", 0, 0, 0, true},
		{"not a time", 0, 0, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			wd, clk, err := ParseResetTime(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if wd != tc.wantWD {
				t.Errorf("weekday: got %v, want %v", wd, tc.wantWD)
			}
			if clk.Hour != tc.wantHour {
				t.Errorf("hour: got %d, want %d", clk.Hour, tc.wantHour)
			}
			if clk.Minute != tc.wantMinute {
				t.Errorf("minute: got %d, want %d", clk.Minute, tc.wantMinute)
			}
		})
	}
}
