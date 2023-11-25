package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/olivercullimore/geo-energy-data-client"
)

func getMetricSeries(name string, value float64) datadogV2.MetricSeries {
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

func getMeterData(ctx context.Context, logger *log.Logger, geoUsername, geoPassword string, datadogMetricsApi *datadogV2.MetricsApi, isPeriodicData bool) {
	accessToken, err := geo.GetAccessToken(geoUsername, geoPassword)
	if err != nil {
		logger.Fatal(err)
	}

	// Get device data to get the system ID
	deviceData, err := geo.GetDeviceData(accessToken)
	if err != nil {
		logger.Fatal(err)
	}
	// Set system ID
	geoSystemID := deviceData.SystemDetails[0].SystemID

	allSeries := []datadogV2.MetricSeries{}

	if isPeriodicData {
		// Get periodic meter data
		periodicData, err := geo.GetPeriodicMeterData(accessToken, geoSystemID)
		if err != nil {
			logger.Fatal(err)
		}

		for _, v := range periodicData.TotalConsumptionList {
			if v.ValueAvailable {
				name := "energy.periodic." + strings.ToLower(v.CommodityType)
				allSeries = append(allSeries, getMetricSeries(name, v.TotalConsumption))
			}
		}

	} else {
		// Get live meter data
		liveData, err := geo.GetLiveMeterData(accessToken, geoSystemID)
		if err != nil {
			logger.Fatal(err)
		}

		for _, v := range liveData.Power {
			if v.ValueAvailable {
				name := "energy.live." + strings.ToLower(v.Type)
				allSeries = append(allSeries, getMetricSeries(name, v.Watts))
			}
		}
	}

	allSeriesBytes, _ := json.Marshal(allSeries)
	logger.Println(string(allSeriesBytes))

	body := datadogV2.MetricPayload{Series: allSeries}

	_, r, err := datadogMetricsApi.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())

	if err != nil {
		logger.Printf("Error when calling `MetricsApi.SubmitMetrics`: %v\n", err)
		logger.Printf("Full HTTP response: %v\n", r)
	}
}

func scheduler(ctx context.Context, logger *log.Logger, tickLive, tickPeriodic *time.Ticker, geoUsername, geoPassword string, datadogMetricsApi *datadogV2.MetricsApi) {
	getMeterData(ctx, logger, geoUsername, geoPassword, datadogMetricsApi, false)
	for {
		select {
		case <-tickLive.C:
			getMeterData(ctx, logger, geoUsername, geoPassword, datadogMetricsApi, false)
		case <-tickPeriodic.C:
			getMeterData(ctx, logger, geoUsername, geoPassword, datadogMetricsApi, true)
		}
	}
}

func main() {
	ctx := datadog.NewDefaultContext(context.Background())
	logger := log.New(os.Stdout, "", log.LstdFlags)
	configuration := datadog.NewConfiguration()
	datadogApiClient := datadog.NewAPIClient(configuration)
	datadogMetricsApi := datadogV2.NewMetricsApi(datadogApiClient)

	liveInterval := 10
	periodicInterval := 300

	tickLive := time.NewTicker(time.Second * time.Duration(liveInterval))
	tickPeriodic := time.NewTicker(time.Second * time.Duration(periodicInterval))

	geoUsername := os.Getenv("GEO_USERNAME")
	geoPassword := os.Getenv("GEO_PASSWORD")

	go scheduler(ctx, logger, tickLive, tickPeriodic, geoUsername, geoPassword, datadogMetricsApi)

	// Wait for a SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
