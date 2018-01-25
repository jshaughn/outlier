package scrape

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Scrape struct {
	Endpoint string
}

var (
	nelsonRules = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nelson_rule",
			Help: "Nelson Rule Violation.",
		},
		[]string{"rule", "ts"},
	)
	responseTimes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "response_time",
			Help: "Response times (for testing only).",
		},
		[]string{"variance"},
	)
)

func (s *Scrape) Add(rule, query string, val float64) {
	nelsonRules.WithLabelValues(rule, query).Add(val)
}

func (s *Scrape) Start() {
	// Register the reported metrics
	prometheus.MustRegister(nelsonRules)
	prometheus.MustRegister(responseTimes)

	// generate values every 5s, start stable and then add variance...
	go func() {
		rand.Seed(time.Now().Unix())
		i := 0
		for {
			var stable, wild int64
			switch {
			case i <= 50:
				stable = rand.Int63n(100)
				wild = rand.Int63n(100)
			default:
				stable = rand.Int63n(50) + 25
				wild = rand.Int63n(125)
			}
			//fmt.Printf("responseTimes: stable=%v, wild=%v\n", stable, wild)
			responseTimes.WithLabelValues("stable").Set(float64(stable))
			responseTimes.WithLabelValues("wild").Set(float64(wild))
			i++
			time.Sleep(time.Duration(5 * time.Second))
		}
	}()

	// Expose the registered metrics via HTTP.
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(s.Endpoint, nil))
}
