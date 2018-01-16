// nelson.go
package nelson

import (
	"container/list"
	"math"
	"sort"
	"time"

	"github.com/gonum/stat"
)

// MaxSamples indicates the max number of Samples needed to evaluate any Rule.
// Rule7 requires the most Samples, 15.
const MaxSamples = 15

type Rule string

var Rule1 Rule = "One point is more than 3 standard deviations from the mean."
var Rule2 Rule = "Nine (or more) points in a row are on the same side of the mean."
var Rule3 Rule = "Six (or more) points in a row are continually increasing (or decreasing)."
var Rule4 Rule = "Fourteen (or more) points in a row alternate in direction, increasing then decreasing."
var Rule5 Rule = "At least 2 of 3 points in a row are > 2 standard deviations from the mean in the same direction."
var Rule6 Rule = "At least 4 of 5 points in a row are > 1 standard deviation from the mean in the same direction."
var Rule7 Rule = "Fifteen points in a row are all within 1 standard deviation of the mean on either side of the mean."
var Rule8 Rule = "Eight points in a row exist, but none within 1 standard deviation of the mean and the points are in both directions from the mean."

func (n Rule) Description() string {
	return string(n)
}

type Sample interface {
	Time() time.Time
	Value() float64
}

type statistics struct {
	ready bool
	// number of samples required to determine mean and stddev
	sampleSize        int
	numSamples        int
	values            []float64
	mean              float64
	standardDeviation float64
	twoDeviations     float64
	threeDeviations   float64
}

func newStatistics(sampleSize int) statistics {
	return statistics{
		sampleSize: sampleSize,
		values:     make([]float64, sampleSize),
	}
}

func (s *statistics) clear() {
	s.numSamples = 0
	s.values = make([]float64, s.sampleSize)
	s.mean = 0
	s.standardDeviation = 0
	s.twoDeviations = 0
	s.threeDeviations = 0
}

// addSample returns true if stats are ready, false otherwise. Values
// added after stats are ready are ignored.
func (s *statistics) addSample(sample Sample) bool {
	if !s.ready {
		s.values[s.numSamples] = sample.Value()
		s.numSamples++
		if s.numSamples == s.sampleSize {
			s.standardDeviation = stat.StdDev(s.values, nil)
			s.twoDeviations = 2 * s.standardDeviation
			s.threeDeviations = 3 * s.standardDeviation
			s.mean = stat.Mean(s.values, nil)
			s.ready = true
		}
	}
	return s.ready
}

// NelsonData tracks nelson rule evaluations for a particular time series.  Each NelsonData
// can be configured with its own sample size.s for different conditions. The life-cycle of
// the NelsonData is tied to the TS.
type Data struct {
	stats statistics
	// List of Rule Elements indicating currently violated Rules
	Violations *list.List
	// List of Sample Elements backing the current Rule evaluations
	ViolationsData         *list.List
	rule2Count             int
	rule3Count             int
	rule3PreviousSample    *float64
	rule4Count             int
	rule4PreviousSample    *float64
	rule4PreviousDirection string
	// List of Sample.Value() Elements
	rule5LastThree *list.List
	rule5Above     int
	rule5Below     int
	// List of Sample.Value() Elements
	rule6LastFive *list.List
	rule6Above    int
	rule6Below    int
	rule7Count    int
	rule8Count    int
}

func NewData(sampleSize int) Data {
	return Data{
		stats:          newStatistics(sampleSize),
		Violations:     list.New(),
		ViolationsData: list.New(),
		rule5LastThree: list.New(),
		rule6LastFive:  list.New(),
	}
}

func (d *Data) Clear() {
	d.stats.clear()
	d.Violations = d.Violations.Init()
	d.ViolationsData = d.ViolationsData.Init()
	d.rule2Count = 0
	d.rule3Count = 0
	d.rule3PreviousSample = nil
	d.rule4Count = 0
	d.rule4PreviousSample = nil
	d.rule4PreviousDirection = ""
	d.rule5LastThree.Init()
	d.rule5Above = 0
	d.rule5Below = 0
	d.rule6LastFive.Init()
	d.rule6Above = 0
	d.rule6Below = 0
	d.rule7Count = 0
	d.rule8Count = 0
}

func (d *Data) hasViolations() bool {
	return 0 < d.Violations.Len()
}

func (d *Data) AddSamples(samples ...Sample) {
	// sort by time ascending (process oldest first)
	sort.Slice(samples,
		func(i, j int) bool {
			return samples[i].Time().Before(samples[j].Time())
		})

	for _, s := range samples {
		if d.stats.ready {
			d.evaluate(s)
		} else {
			d.stats.addSample(s)
		}
	}
}

func (d *Data) evaluate(s Sample) {
	d.ViolationsData.PushFront(s)
	if d.ViolationsData.Len() > MaxSamples {
		d.ViolationsData.Remove(d.ViolationsData.Back())
	}

	if d.rule1(s.Value()) {
		d.Violations.PushBack(Rule1)
	}
	if d.rule2(s.Value()) {
		d.Violations.PushBack(Rule2)
	}
	if d.rule3(s.Value()) {
		d.Violations.PushBack(Rule3)
	}
	if d.rule4(s.Value()) {
		d.Violations.PushBack(Rule4)
	}
	if d.rule5(s.Value()) {
		d.Violations.PushBack(Rule5)
	}
	if d.rule6(s.Value()) {
		d.Violations.PushBack(Rule6)
	}
	if d.rule7(s.Value()) {
		d.Violations.PushBack(Rule7)
	}
	if d.rule8(s.Value()) {
		d.Violations.PushBack(Rule8)
	}
}

// one point is more than 3 standard deviations from the mean
func (d *Data) rule1(s float64) bool {
	return math.Abs(s-d.stats.mean) > d.stats.threeDeviations
}

// Nine (or more) points in a row are on the same side of the mean
func (d *Data) rule2(s float64) bool {
	if s > d.stats.mean {
		if d.rule2Count > 0 {
			d.rule2Count++
		} else {
			d.rule2Count = 1
		}
	} else {
		if d.rule2Count < 0 {
			d.rule2Count--
		} else {
			d.rule2Count = -1
		}
	}

	return math.Abs(float64(d.rule2Count)) >= 9
}

// Six (or more) points in a row are continually increasing (or decreasing)
func (d *Data) rule3(s float64) bool {
	if nil == d.rule3PreviousSample {
		d.rule3PreviousSample = &s
		d.rule3Count = 0
		return false
	}

	if s > *d.rule3PreviousSample {
		if d.rule3Count > 0 {
			d.rule3Count++
		} else {
			d.rule3Count = 1
		}
	} else if s < *d.rule3PreviousSample {
		if d.rule3Count < 0 {
			d.rule3Count--
		} else {
			d.rule3Count = -1
		}
	} else {
		d.rule3Count = 0
	}

	*d.rule3PreviousSample = s

	return math.Abs(float64(d.rule3Count)) >= 6
}

// Fourteen (or more) points in a row alternate in direction, increasing then decreasing
func (d *Data) rule4(s float64) bool {
	if nil == d.rule4PreviousSample || s == *d.rule4PreviousSample {
		d.rule4PreviousSample = &s
		d.rule4PreviousDirection = "="
		d.rule4Count = 0
		return false
	}

	sampleDirection := ">"
	if s <= *d.rule4PreviousSample {
		sampleDirection = "<"
	}

	if sampleDirection == d.rule4PreviousDirection {
		d.rule4Count = 0
	} else {
		d.rule4Count++
	}

	*d.rule4PreviousSample = s
	d.rule4PreviousDirection = sampleDirection

	return math.Abs(float64(d.rule4Count)) >= 14

}

// At least 2 of 3 points in a row are > 2 standard deviations from the mean in the same direction
func (d *Data) rule5(s float64) bool {

	if d.rule5LastThree.Len() == 3 {
		switch d.rule5LastThree.Remove(d.rule5LastThree.Back()) {
		case ">":
			d.rule5Above--
		case "<":
			d.rule5Below--
		}
	}
	if math.Abs(s-d.stats.mean) > d.stats.twoDeviations {
		if s > d.stats.mean {
			d.rule5Above++
			d.rule5LastThree.PushFront(">")
		} else {
			d.rule5Below++
			d.rule5LastThree.PushFront("<")
		}
	} else {
		d.rule5LastThree.PushFront("")
	}

	return d.rule5Above >= 2 || d.rule5Below >= 2
}

// At least 4 of 5 points in a row are > 1 standard deviation from the mean in the same direction
func (d *Data) rule6(s float64) bool {

	if d.rule6LastFive.Len() == 5 {
		switch d.rule6LastFive.Remove(d.rule6LastFive.Back()) {
		case ">":
			d.rule6Above--
		case "<":
			d.rule6Below--
		}
	}

	if math.Abs(s-d.stats.mean) > d.stats.standardDeviation {
		if s > d.stats.mean {
			d.rule6Above++
			d.rule6LastFive.PushFront(">")
		} else {
			d.rule6Below++
			d.rule6LastFive.PushFront("<")
		}
	} else {
		d.rule6LastFive.PushFront("")
	}

	return d.rule6Above >= 4 || d.rule6Below >= 4
}

// Fifteen points in a row are all within 1 standard deviation of the mean on either side of the mean
// Note: I have my doubts about this one wrt monitored metrics, i think it may not be uncommon to have
// a very steady metric. Minimally, I have taken away the flat-line case where all samples are the mean.
func (d *Data) rule7(s float64) bool {

	if s == d.stats.mean {
		d.rule7Count = 0
		return false
	}

	if math.Abs(s-d.stats.mean) <= d.stats.standardDeviation {
		d.rule7Count++
	} else {
		d.rule7Count = 0
	}

	return d.rule7Count >= 15
}

// Eight points in a row exist, but none within 1 standard deviation of the mean
// and the points are in both directions from the mean
func (d *Data) rule8(s float64) bool {

	if math.Abs(s-d.stats.mean) > d.stats.standardDeviation {
		d.rule8Count++
	} else {
		d.rule8Count = 0
	}

	return d.rule8Count >= 8
}
