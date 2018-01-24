// nelson_test.go
package nelson

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"testing"
)

type testSample struct {
	t int64
	v float64
}

func (ts testSample) Time() int64 {
	return ts.t
}

func (ts testSample) Val() float64 {
	return ts.v
}

var (
	// Use SampleSize=10 and the following samples to establish mean=10 and stddev=2.58199
	statSamples = []Sample{
		testSample{100000, 6.0},
		testSample{101000, 7.0},
		testSample{102000, 8.0},
		testSample{103000, 9.0},
		testSample{104000, 10.0},
		testSample{105000, 10.0},
		testSample{106000, 11.0},
		testSample{107000, 12.0},
		testSample{108000, 13.0},
		testSample{109000, 14.0},
	}
)

func TestStats(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)
	assertEqual(t, "10.0", fmt.Sprintf("%.1f", d.stats.mean))
	assertEqual(t, "2.58199", fmt.Sprintf("%.5f", d.stats.standardDeviation))
}

// violate rule 1 : One point is more than 3 standard deviations from the mean
// 9, 10, [ 18 ], 11
func TestRule1(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 9.0},
		testSample{201000, 10.0},
		testSample{202000, 18.0},
		testSample{203000, 11.0},
	}

	d.AddSamples(testSamples)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule1.Name])
}

// violate rule 2: nine (or more) points in a row are on the same side of the mean
// 9, 11, 10, [ 11, 11, 11, 11, 11, 11, 11, 11, 11 ], 8
func TestRule2(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 9.0},
		testSample{201000, 11.0},
		testSample{202000, 10.0},
		testSample{203000, 11.0},
		testSample{204000, 11.0},
		testSample{205000, 11.0},
		testSample{206000, 11.0},
		testSample{207000, 11.0},
		testSample{208000, 11.0},
		testSample{209000, 11.0},
		testSample{210000, 11.0},
		testSample{211000, 11.0},
		testSample{212000, 8.0},
	}

	d.AddSamples(testSamples)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule2.Name])
}

// violate rule 3: Six (or more) points in a row are continually increasing
// 9.3, [ 7.4, 9.9, 10.0, 10.1, 10.2, 10.3, 10.4 ], 8.1
func TestRule3_1(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 9.3},
		testSample{201000, 7.4},
		testSample{202000, 9.9},
		testSample{203000, 10.0},
		testSample{204000, 10.1},
		testSample{205000, 10.2},
		testSample{206000, 10.3},
		testSample{207000, 10.4},
		testSample{208000, 8.1},
	}

	d.AddSamples(testSamples)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule3.Name])
}

// violate rule 3: Six (or more) points in a row are continually decreasing
// 9.3, [ 10.4, 10.3, 10.2, 10.1, 10, 9.9, 7.4 ], 8.1
func TestRule3_2(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 9.3},
		testSample{201000, 10.4},
		testSample{202000, 10.3},
		testSample{203000, 10.2},
		testSample{204000, 10.1},
		testSample{205000, 10.0},
		testSample{206000, 9.9},
		testSample{207000, 7.4},
		testSample{208000, 8.1},
	}

	d.AddSamples(testSamples)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule3.Name])
}

// violate rule 4: Fourteen (or more) points in a row alternate in direction, increasing then decreasing.
// [ 9.5, 12.6, 9.5, 10.5, 9.5, 10.5, 9.5, 10.5, 9.5, 10.5, 9.5, 10.5, 9.5, 10.5, 9.8 ]
func TestRule4(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 9.5},
		testSample{201000, 12.6},
		testSample{202000, 9.5},
		testSample{203000, 10.5},
		testSample{204000, 9.5},
		testSample{205000, 10.5},
		testSample{206000, 9.5},
		testSample{207000, 10.5},
		testSample{208000, 9.5},
		testSample{209000, 10.5},
		testSample{210000, 9.5},
		testSample{211000, 10.5},
		testSample{212000, 9.5},
	}

	d.AddSamples(testSamples)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{213000, 10.5},
		testSample{214000, 9.8},
	}

	d.AddSamples(testSamples)
	assertEqual(t, true, d.hasViolations())
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule4.Name])
}

// violate rule 5: At least 2 of 3 points in a row are > 2 deviations from the mean, in the same direction.
// [ 4, 16, 4 ]
func TestRule5(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 4.0},
		testSample{201000, 16.0},
	}

	d.AddSamples(testSamples)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{202000, 4.0},
	}

	d.AddSamples(testSamples)
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule5.Name])
}

// violate rule 6: At least 4 of 5 points in a row are > 1 deviation from the mean in the same direction.
// [ 7, 7, 4, 16, 7 ]
func TestRule6(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 7.0},
		testSample{201000, 7.0},
		testSample{202000, 4.0},
		testSample{202000, 16.0},
	}

	d.AddSamples(testSamples)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{203000, 7.0},
	}

	d.AddSamples(testSamples)
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule6.Name])
}

// violate rule 7: Fifteen points in a row are all within 1 deviation of the mean on either side of the mean.
// 9.5, 11.5, 11.5, 9.5, 9.5, 11.5, 11.5, 9.5, 9.5, 11.5, 11.5, 9.5, 9.5, 11.5, 11.5
func TestRule7(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 9.5},
		testSample{201000, 11.5},
		testSample{202000, 11.5},
		testSample{203000, 9.5},
		testSample{204000, 9.5},
		testSample{205000, 11.5},
		testSample{206000, 11.5},
		testSample{207000, 9.5},
		testSample{208000, 9.5},
		testSample{209000, 11.5},
		testSample{210000, 11.5},
		testSample{211000, 9.5},
		testSample{212000, 9.5},
	}

	d.AddSamples(testSamples)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{213000, 11.5},
		testSample{213000, 11.5},
	}

	d.AddSamples(testSamples)
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule7.Name])
}

// violate rule 8: Eight points in a row exist, but none within 1 standard deviation of the mean,
// and the points are in both directions from the mean.
// 7, 13, 7, 13, 7, 13, 7, 13, 9.5, 11.5, 11.5, 9.5, 9.5, 11.5, 11.5
func TestRule8(t *testing.T) {
	d := NewData("test-metric", 10)
	d.AddSamples(statSamples)
	assertEqual(t, true, d.stats.ready)

	testSamples := []Sample{
		testSample{200000, 7.0},
		testSample{201000, 13.0},
		testSample{202000, 7.0},
		testSample{203000, 13.0},
		testSample{204000, 7.0},
		testSample{205000, 13.0},
		testSample{206000, 7.0},
	}

	d.AddSamples(testSamples)
	assertEqual(t, false, d.hasViolations()) // not yet

	testSamples = []Sample{
		testSample{207000, 13.0},
	}

	d.AddSamples(testSamples)
	assertEqual(t, 1, len(d.Violations))
	assertEqual(t, 1, d.Violations[Rule8.Name])
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
