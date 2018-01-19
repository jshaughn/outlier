// main.go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/jshaughn/outlier/nelson"
)

type options struct {
	server     string
	sampleSize int
	offset     time.Duration
	interval   time.Duration
}

func parseFlags() options {
	serverDefault, ok := os.LookupEnv("PROMETHEUS_SERVER")
	if !ok {
		serverDefault = "http://localhost:9090"
	}
	server := flag.String("server", serverDefault, "Prometheus server URL (can be set via PROMETHEUS_SERVER environment variable)")
	sampleSize := flag.String("sampleSize", "50", "Number of data points used to calculate mean, standard deviation, etc.")
	offset := flag.String("offset", "0m", "Offset (Xm, Xh, or Xd) from now to start metric sample collection.")
	interval := flag.String("interval", "30s", "Query interval (Xs")

	flag.Parse()

	return options{
		server:     *server,
		sampleSize: intOption(*sampleSize),
		offset:     durationOption(*offset),
		interval:   durationOption(*interval),
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
		"http_requests_total",
	}
)

func (ts TSExpression) process(o options, api v1.API) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := fmt.Sprintf("%v [%v]", ts, o.interval)
	if o.offset.Minutes() > 0 {
		s = fmt.Sprintf("%v offset %vm", s, o.offset.Minutes())
	}
	fmt.Println("Full TS Expression:", s)
	value, err := api.Query(ctx, s, time.Now())
	checkError(err)

	switch t := value.Type(); t {
	case model.ValVector: // Instant Vector
		fmt.Printf("Handle Instant Vector\n")
		vector := value.(model.Vector)
		for _, sample := range vector {
			fmt.Printf("sample: %v\n", sample)
		}
	case model.ValMatrix: // Range Vector
		fmt.Printf("Handle Range Vector\n")
		matrix := value.(model.Matrix)
		for _, sample := range matrix {
			evaluate(sample, o)
		}
	default:
		fmt.Printf("I don't know about type %v!\n", t)
	}
}

func checkError(err error) {
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

var nelsonMap = make(map[string]nelson.Data)

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
	k := s.String()
	d, ok := nelsonMap[k]
	if !ok {
		fmt.Println("Start tracking TS ", k)
		d = nelson.NewData(s.Metric, o.sampleSize, nelson.CommonRules...)
		nelsonMap[k] = d
	}
	var sps []nelson.Sample = toSamplePairs(s.Values)
	d.AddSamples(sps)
	fmt.Printf("Data: %+v\n", d)
}

func main() {
	options := parseFlags()
	checkError(validateOptions(options))

	config := api.Config{options.server, nil}
	client, err := api.NewClient(config)
	checkError(err)

	api := v1.NewAPI(client)

	for _, ts := range tsExpressions {
		ts.process(options, api)
	}
}
