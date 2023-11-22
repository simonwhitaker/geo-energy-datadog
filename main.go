package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/olivercullimore/geo-energy-data-client"
)

func getMetricSeries(name string, value float64) datadogV2.MetricSeries {
	fmt.Printf("%v: %.3f\n", name, value)
	return datadogV2.MetricSeries{
		Metric: name,
		Type:   datadogV2.METRICINTAKETYPE_GAUGE.Ptr(),
		Points: []datadogV2.MetricPoint{
			{
				Timestamp: datadog.PtrInt64(time.Now().Unix()),
				Value:     datadog.PtrFloat64(value),
			},
		},
		Resources: []datadogV2.MetricResource{
			{
				Name: datadog.PtrString("localhost"),
				Type: datadog.PtrString("host"),
			},
		},
	}
}

func main() {
	ctx := datadog.NewDefaultContext(context.Background())
	configuration := datadog.NewConfiguration()
	datadogApiClient := datadog.NewAPIClient(configuration)
	datadogMetricsApi := datadogV2.NewMetricsApi(datadogApiClient)

	username := os.Getenv("GEO_USERNAME")
	password := os.Getenv("GEO_PASSWORD")

	accessToken, err := geo.GetAccessToken(username, password)
	if err != nil {
		log.Fatal(err)
	}

	// Get device data to get the system ID
	deviceData, err := geo.GetDeviceData(accessToken)
	if err != nil {
		log.Fatal(err)
	}
	// Set system ID
	geoSystemID := deviceData.SystemDetails[0].SystemID

	allSeries := []datadogV2.MetricSeries{}

	// Get live meter data
	liveData, err := geo.GetLiveMeterData(accessToken, geoSystemID)
	if err != nil {
		log.Fatal(err)
	}

	for _, v := range liveData.Power {
		if v.ValueAvailable {
			name := "energy.live." + strings.ToLower(v.Type)
			allSeries = append(allSeries, getMetricSeries(name, v.Watts))
		}
	}

	// Get periodic meter data
	periodicData, err := geo.GetPeriodicMeterData(accessToken, geoSystemID)
	if err != nil {
		log.Fatal(err)
	}

	for _, v := range periodicData.TotalConsumptionList {
		if v.ValueAvailable {
			name := "energy.periodic." + strings.ToLower(v.CommodityType)
			allSeries = append(allSeries, getMetricSeries(name, v.TotalConsumption))
		}
	}

	body := datadogV2.MetricPayload{Series: allSeries}

	resp, r, err := datadogMetricsApi.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `MetricsApi.SubmitMetrics`: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}

	responseContent, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintf(os.Stdout, "Response from `MetricsApi.SubmitMetrics`:\n%s\n", responseContent)

}
