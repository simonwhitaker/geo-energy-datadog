package energy

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func TestGetMetricSeries(t *testing.T) {
	reading := Reading{Commodity: ELECTRICITY, ReadingType: LIVE, Value: 2.0}
	w := NewDatadogWriter("", "", "some-hostname", nil)

	actual := w.getMetricSeries(reading)
	expected := datadogV2.MetricSeries{
		Metric: "energy.live.electricity",
		Type:   datadogV2.METRICINTAKETYPE_GAUGE.Ptr(),
		Points: []datadogV2.MetricPoint{
			{
				// NB: Timestamp might not be the same in expected as in actual.
				Timestamp: datadog.PtrInt64(time.Now().Unix()),
				Value:     datadog.PtrFloat64(2.0),
			},
		},
		Resources: []datadogV2.MetricResource{
			{
				Name: datadog.PtrString("some-hostname"),
				Type: datadog.PtrString("host"),
			},
		},
	}

	if expected.Metric != actual.Metric {
		t.Fatalf("Expected %v, got %v", expected.Metric, actual.Metric)
	}

	if len(expected.Resources) != len(actual.Resources) {
		t.Fatalf("Unexpected number of resources. Expected %d, got %d", len(expected.Resources), len(actual.Resources))
	}

	if *expected.Resources[0].Type != *actual.Resources[0].Type {
		t.Fatalf("Unexpected resource type. Expected %v, got %v", expected.Resources[0].Type, actual.Resources[0].Type)
	}
}
