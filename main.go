// main.go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/jshaughn/outlier/nelson"
	"github.com/jshaughn/outlier/scrape"
)

type options struct {
	server     string
	sampleSize int
	offset     time.Duration
	interval   time.Duration
	endpoint   string
}

func parseFlags() options {
	serverDefault, ok := os.LookupEnv("PROMETHEUS_SERVER")
	if !ok {
		serverDefault = "http://localhost:9090"
	}
	server := flag.String("server", serverDefault, "Prometheus server URL (can be set via PROMETHEUS_SERVER environment variable)")
	sampleSize := flag.String("sampleSize", "50", "Number of data points used to calculate mean, standard deviation, etc.")
	offset := flag.String("offset", "0m", "Offset (Xm, Xh, or Xd) from now to start metric sample collection.")
	interval := flag.String("interval", "30s", "Query interval (Xs). Recommended 2 times the scrape interval.")
	endpoint := flag.String("endpoint", ":8080", "The scrape endpoint")

	flag.Parse()

	return options{
		server:     *server,
		sampleSize: intOption(*sampleSize),
		offset:     durationOption(*offset),
		interval:   durationOption(*interval),
		endpoint:   *endpoint,
	}
}

func intOption(option string) int {
	val, err := strconv.Atoi(option)
	checkError(err)
	return val
}

func durationOption(option string) time.Duration {
	val, err := time.ParseDuration(option)
	checkError(err)
	return val
}

func validateOptions(options options) error {
	fmt.Printf("Options: %+v\n", options)

	if options.sampleSize <= 0 {
		return errors.New("SampleSize must be > 0")
	}
	if options.server == "" {
		return errors.New("Server must be set")
	}

	return nil
}

type TSExpression string

var (
	tsExpressions = []TSExpression{
		"response_time",
	}
)

// process() is expected to execute as a goroutine
func (ts TSExpression) process(o options, api v1.API, wg sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	queryTime := time.Now()
	if o.offset.Seconds() > 0 {
		queryTime = queryTime.Add(-o.offset)
	}

	query := fmt.Sprintf("%v [%v]", ts, o.interval)

	for {
		ts.query(query, queryTime, api, o)
		time.Sleep(o.interval)
		queryTime = queryTime.Add(o.interval)
	}
}

// TF is the TimeFormat for printing timestamp
const TF = "2006-01-02 15:04:05"

func (ts TSExpression) query(query string, queryTime time.Time, api v1.API, o options) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Executing query %s @%s (now=%v)\n", query, queryTime.Format(TF), time.Now().Format(TF))

	value, err := api.Query(ctx, query, queryTime)
	checkError(err)

	switch t := value.Type(); t {
	case model.ValVector: // Instant Vector
		fmt.Printf("Handle Instant Vector\n")
		vector := value.(model.Vector)
		for _, sample := range vector {
			fmt.Printf("sample: %v\n", sample)
		}
	case model.ValMatrix: // Range Vector
		matrix := value.(model.Matrix)
		//fmt.Printf("Handle Range Vector, matrix len=%v\n", len(matrix))
		for _, sample := range matrix {
			evaluate(sample, o)
		}
	default:
		fmt.Printf("No handling for type %v!\n", t)
	}
}

func checkError(err error) {
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// nelsonMap is concurrent key=metric string, value=*nelson.Data
var nelsonMap sync.Map

type SamplePair model.SamplePair

// Time() returns ms since epoch (i.e. unix timestamp)
func (sp SamplePair) Time() int64 {
	return int64(sp.Timestamp)
}

func (sp SamplePair) Val() float64 {
	return float64(sp.Value)
}

func toSamplePairs(in []model.SamplePair) (out []nelson.Sample) {
	out = make([]nelson.Sample, len(in))
	for i, v := range in {
		out[i] = SamplePair(v)
	}
	return out
}

func evaluate(s *model.SampleStream, o options) {
	//nelsonMap.Range(
	//	func(k interface{}, v interface{}) bool {
	//		fmt.Println("MapKey:", k)
	//		return true
	//	})

	k := s.Metric.String()
	result, ok := nelsonMap.Load(k)
	var d *nelson.Data
	if !ok {
		fmt.Println("Start tracking TS ", k)
		ds := nelson.NewData(s.Metric, o.sampleSize, nelson.CommonRules...)
		d = &ds
		nelsonMap.Store(k, d)
	} else {
		d = result.(*nelson.Data)
	}
	var sps []nelson.Sample = toSamplePairs(s.Values)
	d.AddSamples(sps)
	fmt.Printf("Data: %+v\n", d)
}

func main() {
	options := parseFlags()
	checkError(validateOptions(options))

	ep := scrape.Scrape{options.endpoint}
	go ep.Start()

	config := api.Config{options.server, nil}
	client, err := api.NewClient(config)
	checkError(err)

	api := v1.NewAPI(client)

	var wg sync.WaitGroup

	for _, ts := range tsExpressions {
		ts.process(options, api, wg)
	}

	wg.Wait()
}
