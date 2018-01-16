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
)

type options struct {
	server       string
	sampleMin    int
	samplePeriod time.Duration
	sampleOffset time.Duration
	interval     time.Duration
}

func main() {
	options := parseFlags()
	checkError(validateOptions(options))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := api.Config{options.server, nil}
	client, err := api.NewClient(config)
	checkError(err)

	api := v1.NewAPI(client)
	value, err := api.Query(ctx, "http_requests_total [1m]", time.Now())
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
			fmt.Printf("sample: %v\n", sample)
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

func parseFlags() options {
	serverDefault, ok := os.LookupEnv("PROMETHEUS_SERVER")
	if !ok {
		serverDefault = "http://localhost:9090"
	}
	server := flag.String("server", serverDefault, "Prometheus server URL (can be set via PROMETHEUS_SERVER environment variable)")
	sampleMin := flag.String("sampleMin", "50", "Minimum number of data points used to calculate mean, standard deviation, etc.")
	samplePeriod := flag.String("samplePeriod", "24h", "Period of data used to calculate mean, standard deviation, etc.")
	sampleOffset := flag.String("sampleOffset", "0h", "Offset from now used as endpoint for sample query.")
	interval := flag.String("interval", "15s", "Query interval.")

	flag.Parse()

	return options{
		server:       *server,
		sampleMin:    intOption(*sampleMin),
		samplePeriod: durationOption(*samplePeriod),
		sampleOffset: durationOption(*sampleOffset),
		interval:     durationOption(*interval),
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

	if options.sampleMin <= 0 {
		return errors.New("-sampleMin must be > 0")
	}
	if options.server == "" {
		return errors.New("-server must be set")
	}

	return nil
}
