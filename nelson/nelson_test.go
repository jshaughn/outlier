// nelson_test.go
package nelson

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"testing"
	"time"
)

type testSample struct {
	t time.Time
	v float64
}

func (ts testSample) Time() time.Time {
	return ts.t
}

func (ts testSample) Value() float64 {
	return ts.v
}

var (
	// Use SampleSize=10 and the following samples to establish mean=10 and stddev=2.58199
	statSamples = []Sample{
		testSample{time.Unix(100000, 0), 6.0},
		testSample{time.Unix(101000, 0), 7.0},
		testSample{time.Unix(102000, 0), 8.0},
		testSample{time.Unix(103000, 0), 9.0},
		testSample{time.Unix(104000, 0), 10.0},
		testSample{time.Unix(105000, 0), 10.0},
		testSample{time.Unix(106000, 0), 11.0},
		testSample{time.Unix(107000, 0), 12.0},
		testSample{time.Unix(108000, 0), 13.0},
		testSample{time.Unix(109000, 0), 14.0},
	}
)

func TestStats(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)
	assertEqual(t, "10.0", fmt.Sprintf("%.1f", d.stats.mean))
	assertEqual(t, "2.58199", fmt.Sprintf("%.5f", d.stats.standardDeviation))
}

// violate rule 1 : One point is more than 3 standard deviations from the mean
// 9, 10, [ 18 ], 11
func TestRule1(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 9.0},
		testSample{time.Unix(201000, 0), 10.0},
		testSample{time.Unix(202000, 0), 18.0},
		testSample{time.Unix(203000, 0), 11.0},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule1, d.Violations.Front().Value)
}

// violate rule 2: nine (or more) points in a row are on the same side of the mean
// 9, 11, 10, [ 11, 11, 11, 11, 11, 11, 11, 11, 11 ], 8
func TestRule2(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 9.0},
		testSample{time.Unix(201000, 0), 11.0},
		testSample{time.Unix(202000, 0), 10.0},
		testSample{time.Unix(203000, 0), 11.0},
		testSample{time.Unix(204000, 0), 11.0},
		testSample{time.Unix(205000, 0), 11.0},
		testSample{time.Unix(206000, 0), 11.0},
		testSample{time.Unix(207000, 0), 11.0},
		testSample{time.Unix(208000, 0), 11.0},
		testSample{time.Unix(209000, 0), 11.0},
		testSample{time.Unix(210000, 0), 11.0},
		testSample{time.Unix(211000, 0), 11.0},
		testSample{time.Unix(212000, 0), 8.0},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule2, d.Violations.Front().Value)
}

// violate rule 3: Six (or more) points in a row are continually increasing
// 9.3, [ 7.4, 9.9, 10.0, 10.1, 10.2, 10.3, 10.4 ], 8.1
func TestRule3_1(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 9.3},
		testSample{time.Unix(201000, 0), 7.4},
		testSample{time.Unix(202000, 0), 9.9},
		testSample{time.Unix(203000, 0), 10.0},
		testSample{time.Unix(204000, 0), 10.1},
		testSample{time.Unix(205000, 0), 10.2},
		testSample{time.Unix(206000, 0), 10.3},
		testSample{time.Unix(207000, 0), 10.4},
		testSample{time.Unix(208000, 0), 8.1},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule3, d.Violations.Front().Value)
}

// violate rule 3: Six (or more) points in a row are continually decreasing
// 9.3, [ 10.4, 10.3, 10.2, 10.1, 10, 9.9, 7.4 ], 8.1
func TestRule3_2(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 9.3},
		testSample{time.Unix(201000, 0), 10.4},
		testSample{time.Unix(202000, 0), 10.3},
		testSample{time.Unix(203000, 0), 10.2},
		testSample{time.Unix(204000, 0), 10.1},
		testSample{time.Unix(205000, 0), 10.0},
		testSample{time.Unix(206000, 0), 9.9},
		testSample{time.Unix(207000, 0), 7.4},
		testSample{time.Unix(208000, 0), 8.1},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule3, d.Violations.Front().Value)
}

// violate rule 4: Fourteen (or more) points in a row alternate in direction, increasing then decreasing.
// [ 9.5, 12.6, 9.5, 10.5, 9.5, 10.5, 9.5, 10.5, 9.5, 10.5, 9.5, 10.5, 9.5, 10.5, 9.8 ]
func TestRule4(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 9.5},
		testSample{time.Unix(201000, 0), 12.6},
		testSample{time.Unix(202000, 0), 9.5},
		testSample{time.Unix(203000, 0), 10.5},
		testSample{time.Unix(204000, 0), 9.5},
		testSample{time.Unix(205000, 0), 10.5},
		testSample{time.Unix(206000, 0), 9.5},
		testSample{time.Unix(207000, 0), 10.5},
		testSample{time.Unix(208000, 0), 9.5},
		testSample{time.Unix(209000, 0), 10.5},
		testSample{time.Unix(210000, 0), 9.5},
		testSample{time.Unix(211000, 0), 10.5},
		testSample{time.Unix(212000, 0), 9.5},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{time.Unix(213000, 0), 10.5},
		testSample{time.Unix(214000, 0), 9.8},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule4, d.Violations.Front().Value)
}

// violate rule 5: At least 2 of 3 points in a row are > 2 deviations from the mean, in the same direction.
// [ 4, 16, 4 ]
func TestRule5(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 4.0},
		testSample{time.Unix(201000, 0), 16.0},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{time.Unix(202000, 0), 4.0},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule5, d.Violations.Front().Value)
}

// violate rule 6: At least 4 of 5 points in a row are > 1 deviation from the mean in the same direction.
// [ 7, 7, 4, 16, 7 ]
func TestRule6(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 7.0},
		testSample{time.Unix(201000, 0), 7.0},
		testSample{time.Unix(202000, 0), 4.0},
		testSample{time.Unix(202000, 0), 16.0},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{time.Unix(203000, 0), 7.0},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule6, d.Violations.Front().Value)
}

// violate rule 7: Fifteen points in a row are all within 1 deviation of the mean on either side of the mean.
// 9.5, 11.5, 11.5, 9.5, 9.5, 11.5, 11.5, 9.5, 9.5, 11.5, 11.5, 9.5, 9.5, 11.5, 11.5
func TestRule7(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 9.5},
		testSample{time.Unix(201000, 0), 11.5},
		testSample{time.Unix(202000, 0), 11.5},
		testSample{time.Unix(203000, 0), 9.5},
		testSample{time.Unix(204000, 0), 9.5},
		testSample{time.Unix(205000, 0), 11.5},
		testSample{time.Unix(206000, 0), 11.5},
		testSample{time.Unix(207000, 0), 9.5},
		testSample{time.Unix(208000, 0), 9.5},
		testSample{time.Unix(209000, 0), 11.5},
		testSample{time.Unix(210000, 0), 11.5},
		testSample{time.Unix(211000, 0), 9.5},
		testSample{time.Unix(212000, 0), 9.5},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{time.Unix(213000, 0), 11.5},
		testSample{time.Unix(213000, 0), 11.5},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule7, d.Violations.Front().Value)
}

// violate rule 8: Eight points in a row exist, but none within 1 standard deviation of the mean,
// and the points are in both directions from the mean.
// 7, 13, 7, 13, 7, 13, 7, 13, 9.5, 11.5, 11.5, 9.5, 9.5, 11.5, 11.5
func TestRule8(t *testing.T) {
	d := NewData(10)
	d.AddSamples(statSamples...)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{time.Unix(200000, 0), 7.0},
		testSample{time.Unix(201000, 0), 13.0},
		testSample{time.Unix(202000, 0), 7.0},
		testSample{time.Unix(203000, 0), 13.0},
		testSample{time.Unix(204000, 0), 7.0},
		testSample{time.Unix(205000, 0), 13.0},
		testSample{time.Unix(206000, 0), 7.0},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{time.Unix(207000, 0), 13.0},
	}

	d.AddSamples(testSamples...)
	assertEqual(t, 1, d.Violations.Len())
	assertEqual(t, Rule8, d.Violations.Front().Value)
}

func assertEqual(t *testing.T, e interface{}, v interface{}) {
	if reflect.TypeOf(e) != reflect.TypeOf(v) {
		debug.PrintStack()
		t.Fatal(fmt.Sprintf("Expected |%v|, Got |%v|", reflect.TypeOf(e), reflect.TypeOf(v)))
	}
	if e != v {
		debug.PrintStack()
		t.Fatal(fmt.Sprintf("Expected |%v|, Got |%v|", e, v))
	}
}
