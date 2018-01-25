// nelson.go
package nelson

import (
	"container/list"
	"fmt"
	"math"

	"github.com/gonum/stat"
)

type Rule struct {
	Name        string
	Description string
	f           func(d *Data, v float64) bool
}

var Rule1 = Rule{
	"Rule1",
	"One point is more than 3 standard deviations from the mean.",
	(*Data).rule1,
}
var Rule2 = Rule{
	"Rule2",
	"Nine (or more) points in a row are on the same side of the mean.",
	(*Data).rule2,
}
var Rule3 = Rule{
	"Rule3",
	"Six (or more) points in a row are continually increasing (or decreasing).",
	(*Data).rule3,
}
var Rule4 = Rule{
	"Rule4",
	"Fourteen (or more) points in a row alternate in direction, increasing then decreasing.",
	(*Data).rule4,
}
var Rule5 = Rule{
	"Rule5",
	"At least 2 of 3 points in a row are > 2 standard deviations from the mean in the same direction.",
	(*Data).rule5,
}
var Rule6 = Rule{
	"Rule6",
	"At least 4 of 5 points in a row are > 1 standard deviation from the mean in the same direction.",
	(*Data).rule6,
}
var Rule7 = Rule{
	"Rule7",
	"Fifteen points in a row are all within 1 standard deviation of the mean on either side of the mean.",
	(*Data).rule7,
}
var Rule8 = Rule{
	"Rule8",
	"Eight points in a row exist, but none within 1 standard deviation of the mean and the points are in both directions from the mean.",
	(*Data).rule8,
}

func (r Rule) String() string {
	return r.Name
}

// CommonRules includes all rules other than: Rule7
var CommonRules = []Rule{Rule1, Rule2, Rule3, Rule4, Rule5, Rule6, Rule8}

// AllRules is not recommended for metrics with little to no variance when well-behaved
var AllRules = []Rule{Rule1, Rule2, Rule3, Rule4, Rule5, Rule6, Rule7, Rule8}

// MaxSamples indicates the max number of Samples needed to evaluate any Rule.
// Rule7 requires the most Samples, 15.
const MaxSamples = 15

type Sample interface {
	Time() int64 // unix time in ms
	Val() float64
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

func (s statistics) String() string {
	if !s.ready {
		return fmt.Sprintf("Waiting on [%v] samples", s.sampleSize-s.numSamples)
	}
	return fmt.Sprintf("mean=%.2f, stddev=%.2f, twodev=%.2f, threedev=%.2f",
		s.mean, s.standardDeviation, s.twoDeviations, s.threeDeviations)
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
		s.values[s.numSamples] = sample.Val()
		s.numSamples++
		if s.numSamples == s.sampleSize {
			s.mean = stat.Mean(s.values, nil)
			s.standardDeviation = stat.StdDev(s.values, nil)
			s.twoDeviations = 2 * s.standardDeviation
			s.threeDeviations = 3 * s.standardDeviation
			s.ready = true
		}
	}
	return s.ready
}

// Data tracks nelson rule evaluations for a particular time series.  Each Data
// can be configured with its own sample size and rule set. The life-cycle of
// Data should be tied to the TS.
type Data struct {
	Metric     interface{}
	Violations map[string]int
	// List of Sample Elements backing the current Rule evaluations
	ViolationsData *list.List
	Rules          []Rule
	stats          statistics
	// List of Rule Elements indicating currently violated Rules
	rule2Count             int
	rule3Count             int
	rule3PreviousSample    *float64
	rule4Count             int
	rule4PreviousSample    *float64
	rule4PreviousDirection string
	// List of Sample.Value() Elements
	rule5LastThree *list.List
	// List of Sample.Value() Elements
	rule6LastFive *list.List
	rule7Count    int
	rule8Count    int
}

func NewData(m interface{}, sampleSize int, rules ...Rule) Data {
	if nil == rules {
		rules = AllRules
	}

	return Data{
		Metric:         m,
		Rules:          rules,
		Violations:     make(map[string]int),
		ViolationsData: list.New(),
		rule5LastThree: list.New(),
		rule6LastFive:  list.New(),
		stats:          newStatistics(sampleSize),
	}
}

func (d Data) String() string {
	if len(d.Violations) == 0 {
		return fmt.Sprintf("%v:\n\tNo Violations, stats:%+v", d.Metric, d.stats)
	}

	var vr, comma string
	for k, v := range d.Violations {
		vr += fmt.Sprintf("%s%s(%v)", comma, k, v)
		comma = ","
	}
	vd := "["
	comma = ""
	for e := d.ViolationsData.Front(); e != nil; e = e.Next() {
		vd += fmt.Sprintf("%s%.2f", comma, e.Value.(Sample).Val())
		comma = ","
	}
	vd += "]"

	return fmt.Sprintf("%v:\n\tviolations: %v\n\tstats: %v\n\tvalues: %v", d.Metric, vr, d.stats, vd)
}

func (d *Data) Clear() {
	d.stats.clear()
	d.Violations = make(map[string]int)
	d.ViolationsData = d.ViolationsData.Init()
	d.rule2Count = 0
	d.rule3Count = 0
	d.rule3PreviousSample = nil
	d.rule4Count = 0
	d.rule4PreviousSample = nil
	d.rule4PreviousDirection = ""
	d.rule5LastThree.Init()
	d.rule6LastFive.Init()
	d.rule7Count = 0
	d.rule8Count = 0
}

func (d *Data) hasViolations() bool {
	return len(d.Violations) > 0
}

func (d *Data) AddSample(s Sample) map[string]bool {
	if d.stats.ready {
		return d.evaluate(s)
	}
	d.stats.addSample(s)
	return nil
}

func (d *Data) evaluate(s Sample) (result map[string]bool) {
	d.ViolationsData.PushFront(s)
	if d.ViolationsData.Len() > MaxSamples {
		d.ViolationsData.Remove(d.ViolationsData.Back())
	}

	result = make(map[string]bool)
	for _, r := range d.Rules {
		violation := r.f(d, s.Val())
		result[r.Name] = violation
		if violation {
			fmt.Printf("Violation! %s %v\n", r.Name, d.Metric)
			d.Violations[r.Name] += 1
		}
	}

	return result
}

// one point is more than 3 standard deviations from the mean
func (d *Data) rule1(s float64) bool {
	if d.stats.standardDeviation == 0.0 {
		return false
	}

	return math.Abs(s-d.stats.mean) > d.stats.threeDeviations
}

// Nine (or more) points in a row are on the same side of the mean
func (d *Data) rule2(s float64) bool {
	switch {
	case s > d.stats.mean:
		if d.rule2Count > 0 {
			d.rule2Count++
		} else {
			d.rule2Count = 1
		}
	case s < d.stats.mean:
		if d.rule2Count < 0 {
			d.rule2Count--
		} else {
			d.rule2Count = -1
		}
	default:
		d.rule2Count = 0
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
	if d.stats.standardDeviation == 0.0 {
		return false
	}

	if math.Abs(s-d.stats.mean) > d.stats.twoDeviations {
		if s > d.stats.mean {
			d.rule5LastThree.PushFront(">")
		} else {
			d.rule5LastThree.PushFront("<")
		}
	} else {
		d.rule5LastThree.PushFront("")
	}

	if d.rule5LastThree.Len() > 3 {
		d.rule5LastThree.Remove(d.rule5LastThree.Back())
	}

	var above, below int
	for e := d.rule5LastThree.Front(); e != nil; e = e.Next() {
		switch e.Value.(string) {
		case ">":
			above++
		case "<":
			below++
		}
	}

	return above >= 2 || below >= 2
}

// At least 4 of 5 points in a row are > 1 standard deviation from the mean in the same direction
func (d *Data) rule6(s float64) bool {
	if d.stats.standardDeviation == 0.0 {
		return false
	}

	if math.Abs(s-d.stats.mean) > d.stats.standardDeviation {
		if s > d.stats.mean {
			d.rule6LastFive.PushFront(">")
		} else {
			d.rule6LastFive.PushFront("<")
		}
	} else {
		d.rule6LastFive.PushFront("")
	}

	if d.rule6LastFive.Len() > 5 {
		d.rule6LastFive.Remove(d.rule6LastFive.Back())
	}

	var above, below int
	for e := d.rule6LastFive.Front(); e != nil; e = e.Next() {
		switch e.Value.(string) {
		case ">":
			above++
		case "<":
			below++
		}
	}

	return above >= 4 || below >= 4
}

// Fifteen points in a row are all within 1 standard deviation of the mean on either side of the mean
// Note: I have my doubts about this one wrt monitored metrics, i think it may not be uncommon to have
// a very steady metric. Minimally, I have taken away the flat-line case where all samples are the mean.
func (d *Data) rule7(s float64) bool {
	if d.stats.standardDeviation == 0.0 {
		return false
	}

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
	if d.stats.standardDeviation == 0.0 {
		return false
	}

	if math.Abs(s-d.stats.mean) > d.stats.standardDeviation {
		d.rule8Count++
	} else {
		d.rule8Count = 0
	}

	return d.rule8Count >= 8
}
