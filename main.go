package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/simonwhitaker/geo-energy-datadog/energy"
)

type ReadingMode int

const (
	LIVE ReadingMode = 1 << iota
	PERIODIC
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

func getMeterData(ctx context.Context, logger *log.Logger, reader energy.EnergyDataReader, datadogMetricsApi *datadogV2.MetricsApi, mode ReadingMode) {
	allSeries := []datadogV2.MetricSeries{}

	if mode&PERIODIC != 0 {
		// Get periodic meter data
		readings, err := reader.GetMeterReadings()
		if err != nil {
			logger.Fatal(err)
		}

		for _, r := range readings {
			var key string = fmt.Sprintf("energy.periodic.%v", r.Commodity)
			allSeries = append(allSeries, getMetricSeries(key, r.Value))
		}
	}
	if mode&LIVE != 0 {
		// Get live meter data
		readings, err := reader.GetLiveReadings()
		if err != nil {
			logger.Fatal(err)
		}

		for _, r := range readings {
			var key string = fmt.Sprintf("energy.live.%v", r.Commodity)
			allSeries = append(allSeries, getMetricSeries(key, r.Value))
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

func scheduler(ctx context.Context, logger *log.Logger, reader energy.EnergyDataReader, tickLive, tickPeriodic *time.Ticker, datadogMetricsApi *datadogV2.MetricsApi) {
	getMeterData(ctx, logger, reader, datadogMetricsApi, LIVE|PERIODIC)
	for {
		select {
		case <-tickLive.C:
			getMeterData(ctx, logger, reader, datadogMetricsApi, LIVE)
		case <-tickPeriodic.C:
			getMeterData(ctx, logger, reader, datadogMetricsApi, PERIODIC)
		}
	}
}

func main() {
	ctx := datadog.NewDefaultContext(context.Background())
	logger := log.New(os.Stdout, "", log.LstdFlags)

	geoUsername := os.Getenv("GEO_USERNAME")
	geoPassword := os.Getenv("GEO_PASSWORD")
	reader := energy.NewGeoEnergyDataReader(geoUsername, geoPassword)

	configuration := datadog.NewConfiguration()
	datadogApiClient := datadog.NewAPIClient(configuration)
	datadogMetricsApi := datadogV2.NewMetricsApi(datadogApiClient)

	liveInterval := 10
	periodicInterval := 300

	tickLive := time.NewTicker(time.Second * time.Duration(liveInterval))
	tickPeriodic := time.NewTicker(time.Second * time.Duration(periodicInterval))

	go scheduler(ctx, logger, reader, tickLive, tickPeriodic, datadogMetricsApi)

	// Wait for a SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
