package honeycomb

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/honeycombio/libhoney-go"
	"github.com/mble/go-metrics"
)

type Reporter struct {
	Registry      metrics.Registry
	Interval      time.Duration
	WriteKey      string
	Dataset       string
	ServiceName   string
	Source        string
	Percentiles   []float64 // percentiles to report on histogram metrics
	ResetCounters bool

	client  *libhoney.Client
	stopped chan struct{}
}

func NewDefaultReporter(
	registry metrics.Registry,
	writeKey string,
	dataset string,
	serviceName string,
	source string,
	resetCounters bool,
) *Reporter {
	defaultPercentiles := []float64{.5, .95, .99, .999}
	defaultInterval := 60 * time.Second

	return NewReporter(
		registry,
		defaultInterval,
		writeKey,
		dataset,
		serviceName,
		source,
		defaultPercentiles,
		resetCounters,
	)
}

func NewReporter(
	registry metrics.Registry,
	interval time.Duration,
	writeKey string,
	dataset string,
	serviceName string,
	source string,
	percentiles []float64,
	resetCounters bool,
) *Reporter {
	r := &Reporter{
		Registry:      registry,
		Interval:      interval,
		WriteKey:      writeKey,
		Dataset:       dataset,
		ServiceName:   serviceName,
		Source:        source,
		Percentiles:   percentiles,
		ResetCounters: resetCounters,
	}
	r.Init()
	return r
}

func Honeycomb(
	registry metrics.Registry,
	interval time.Duration,
	writeKey string,
	dataset string,
	serviceName string,
	source string,
	percentiles []float64,
	resetCounters bool,
) {
	NewReporter(registry, interval, writeKey, dataset, serviceName, source, percentiles, resetCounters).Run()
}

// Initializes the Honeycomb client.
func (r *Reporter) Init() {
	cfg := libhoney.ClientConfig{
		APIKey:  r.WriteKey,
		Dataset: r.Dataset,
	}

	client, err := libhoney.NewClient(cfg)
	if err != nil {
		panic(fmt.Sprintf("at=libhoney-init err=%q", err))
	}

	r.client = client
}

// Convenience method around libhoney.AddField()
func (r *Reporter) AddField(key string, val interface{}) {
	r.client.AddField(key, val)
}

// Blocks and starts reporting metrics from the provided registry to Honeycomb.
func (r *Reporter) Run() {
	defer r.Stop()
	for {
		select {
		case <-time.After(r.Interval):
			e := r.BuildEvent()
			if err := e.Send(); err != nil {
				log.Printf("at=honeycomb-send err=%q", err)
			}
		case <-r.stopped:
			return
		}
	}
}

// Stops the metrics reporting process and closes any connections to Honeycomb.
func (r *Reporter) Stop() {
	close(r.stopped)
	r.client.Close()
}

func (r *Reporter) buildRequest() map[string]interface{} {
	metricsMap := make(map[string]interface{})
	r.Registry.Each(func(name string, metric interface{}) {
		switch m := metric.(type) {
		case metrics.Counter:
			if m.Count() > 0 {
				metricsMap[fmt.Sprintf("%s.count", name)] = float64(m.Count())
			}

			if r.ResetCounters {
				m.Clear()
			}
		case metrics.Gauge:
			metricsMap[name] = float64(m.Value())
		case metrics.GaugeFloat64:
			metricsMap[name] = float64(m.Value())
		case metrics.Histogram:
			s := m.Sample()
			if m.Count() > 0 {
				metricsMap[fmt.Sprintf("%s.count", name)] = uint64(s.Size())
				metricsMap[fmt.Sprintf("%s.max", name)] = float64(s.Max())
				metricsMap[fmt.Sprintf("%s.mean", name)] = float64(s.Mean())
				metricsMap[fmt.Sprintf("%s.min", name)] = float64(s.Min())
				metricsMap[fmt.Sprintf("%s.sum", name)] = float64(s.Sum())
				for _, p := range r.Percentiles {
					metricsMap[fmt.Sprintf("%s.p%g", name, p*100)] = s.Percentile(p)
				}
			}

			if r.ResetCounters {
				m.Clear()
			}
		case metrics.Meter:
			metricsMap[name] = float64(m.Count())
			metricsMap[fmt.Sprintf("%s.rate.1min", name)] = float64(m.Rate1())
			metricsMap[fmt.Sprintf("%s.rate.5min", name)] = float64(m.Rate5())
			metricsMap[fmt.Sprintf("%s.rate.15min", name)] = float64(m.Rate15())
		case metrics.Timer:
			metricsMap[name] = float64(m.Count())
			if m.Count() > 0 {
				metricsMap[fmt.Sprintf("%s.max", name)] = float64(m.Max())
				metricsMap[fmt.Sprintf("%s.mean", name)] = float64(m.Mean())
				metricsMap[fmt.Sprintf("%s.min", name)] = float64(m.Min())
				metricsMap[fmt.Sprintf("%s.sum", name)] = m.Mean() * float64(m.Count())
				for _, p := range r.Percentiles {
					metricsMap[fmt.Sprintf("%s.p%g", name, p*100)] = m.Percentile(p)
				}
				metricsMap[fmt.Sprintf("%s.rate.1min", name)] = float64(m.Rate1())
				metricsMap[fmt.Sprintf("%s.rate.5min", name)] = float64(m.Rate5())
				metricsMap[fmt.Sprintf("%s.rate.15min", name)] = float64(m.Rate15())
			}

			if r.ResetCounters {
				m.Clear()
			}
		}
	})
	return metricsMap
}

func (r *Reporter) BuildEvent() *libhoney.Event {
	e := r.client.NewEvent()

	req := r.buildRequest()
	_, found := os.LookupEnv("DEBUG")
	if found {
		log.Printf("at=honeycomb-body body=%+v", req)
	}

	e.AddField("source", r.Source)
	e.AddField("service_name", r.ServiceName)
	err := e.Add(req)
	if err != nil {
		log.Printf("at=honeycomb-add err=%q", err)
	}

	return e
}
