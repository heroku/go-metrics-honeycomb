package honeycomb_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/heroku/go-metrics"
	honeycomb "github.com/heroku/go-metrics-honeycomb"
)

func reportersEqual(got, expected *honeycomb.Reporter) bool {
	return got.Registry == expected.Registry &&
		got.Interval == expected.Interval &&
		got.WriteKey == expected.WriteKey &&
		got.Dataset == expected.Dataset &&
		got.ServiceName == expected.ServiceName &&
		got.Source == expected.Source &&
		got.ResetCounters == expected.ResetCounters &&
		reflect.DeepEqual(got.Percentiles, expected.Percentiles)
}

func newReporter(t *testing.T) *honeycomb.Reporter {
	t.Helper()

	return honeycomb.NewDefaultReporter(
		metrics.DefaultRegistry,
		"write-key",
		"example-dataset",
		"app",
		"source",
		false,
	)
}

func TestNewDefaultReporter(t *testing.T) {
	t.Parallel()

	reporter := honeycomb.NewDefaultReporter(
		metrics.DefaultRegistry,
		"write-key",
		"example-dataset",
		"app",
		"source",
		false,
	)

	expected := &honeycomb.Reporter{
		Registry:      metrics.DefaultRegistry,
		Interval:      60 * time.Second,
		WriteKey:      "write-key",
		Dataset:       "example-dataset",
		ServiceName:   "app",
		Source:        "source",
		Percentiles:   []float64{.5, .95, .99, .999},
		ResetCounters: false,
	}

	if !reportersEqual(reporter, expected) {
		t.Errorf("got=%+v expected=%+v", reporter, expected)
	}
}

func TestNewReporter(t *testing.T) {
	t.Parallel()

	reporter := honeycomb.NewReporter(
		metrics.DefaultRegistry,
		15*time.Second,
		"write-key",
		"example-dataset",
		"app",
		"source",
		[]float64{.95, .99},
		true,
	)

	expected := &honeycomb.Reporter{
		Registry:      metrics.DefaultRegistry,
		Interval:      15 * time.Second,
		WriteKey:      "write-key",
		Dataset:       "example-dataset",
		ServiceName:   "app",
		Source:        "source",
		Percentiles:   []float64{.95, .99},
		ResetCounters: true,
	}

	if !reportersEqual(reporter, expected) {
		t.Errorf("got=%+v expected=%+v", reporter, expected)
	}
}

func TestBuildEvent(t *testing.T) {
	reporter := newReporter(t)
	test_counter := metrics.NewCounter()
	test_gauge := metrics.NewGauge()

	reporter.Registry.Register("test_counter", test_counter)
	reporter.Registry.Register("test_gauge", test_gauge)

	test_counter.Inc(10)
	test_gauge.Update(5)

	event := reporter.BuildEvent()

	if event.WriteKey != "write-key" {
		t.Errorf("got=%s expected=%s", event.WriteKey, "write-key")
	}

	if event.Dataset != "example-dataset" {
		t.Errorf("got=%s expected=%s", event.Dataset, "example-dataset")
	}

	expected_fields := map[string]interface{}{"service_name": "app", "source": "source", "test_counter.count": float64(10), "test_gauge": float64(5)}
	fields := event.Fields()

	for k, v := range expected_fields {
		if fields[k] != v {
			t.Errorf("%s: got=%#v, expected=%#v", k, fields[k], v)
		}
	}
}
