package energy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

type DatadogWriter struct {
	apiKey   string
	site     string
	hostname string
	logger   *log.Logger
}

func NewDatadogWriter(apiKey, site, hostname string, logger *log.Logger) DatadogWriter {
	return DatadogWriter{
		apiKey:   apiKey,
		site:     site,
		hostname: hostname,
		logger:   logger,
	}
}

func (w DatadogWriter) WriteReadings(readings []Reading) error {
	ctx := context.WithValue(
		context.Background(),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {
				Key: w.apiKey,
			},
		},
	)
	ctx = context.WithValue(
		ctx,
		datadog.ContextServerVariables,
		map[string]string{
			"site": w.site,
		},
	)
	configuration := datadog.NewConfiguration()
	datadogApiClient := datadog.NewAPIClient(configuration)
	datadogMetricsApi := datadogV2.NewMetricsApi(datadogApiClient)

	allSeries := []datadogV2.MetricSeries{}
	for _, el := range readings {
		allSeries = append(allSeries, w.getMetricSeries(el))
	}

	if w.logger != nil {
		allSeriesBytes, _ := json.Marshal(allSeries)
		w.logger.Println(string(allSeriesBytes))
	}

	body := datadogV2.MetricPayload{Series: allSeries}

	_, _, err := datadogMetricsApi.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())

	return err
}

func (w DatadogWriter) getMetricSeries(r Reading) datadogV2.MetricSeries {
	name := fmt.Sprintf("energy.%v.%v", r.ReadingType, r.Commodity)
	return datadogV2.MetricSeries{
		Metric: name,
		Type:   datadogV2.METRICINTAKETYPE_GAUGE.Ptr(),
		Points: []datadogV2.MetricPoint{
			{
				Timestamp: datadog.PtrInt64(time.Now().Unix()),
				Value:     datadog.PtrFloat64(r.Value),
			},
		},
		Resources: []datadogV2.MetricResource{
			{
				Name: datadog.PtrString(w.hostname),
				Type: datadog.PtrString("host"),
			},
		},
	}
}
