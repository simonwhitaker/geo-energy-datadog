package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/simonwhitaker/geo-energy-datadog/energy"
)

type ReadingMode int

const (
	LIVE ReadingMode = 1 << iota
	PERIODIC
)

func getMeterData(logger *log.Logger, reader energy.EnergyDataReader, writers []energy.EnergyDataWriter, mode ReadingMode) {
	allReadings := []energy.Reading{}

	if mode&PERIODIC != 0 {
		// Get periodic meter data
		readings, err := reader.GetMeterReadings()
		if err != nil {
			logger.Fatal(err)
		}
		allReadings = append(allReadings, readings...)
	}
	if mode&LIVE != 0 {
		// Get live meter data
		readings, err := reader.GetLiveReadings()
		if err != nil {
			logger.Fatal(err)
		}

		allReadings = append(allReadings, readings...)
	}

	for _, w := range writers {
		err := w.WriteReadings(allReadings)
		if err != nil {
			logger.Fatal(err)
		}
	}
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Configure reader
	geoUsername := os.Getenv("GEO_USERNAME")
	geoPassword := os.Getenv("GEO_PASSWORD")
	reader := energy.NewGeoEnergyDataReader(geoUsername, geoPassword)

	// Configure writers
	writers := []energy.EnergyDataWriter{
		energy.NewLoggerWriter(logger),
	}

	if datadogApiKey, ok := os.LookupEnv("DD_API_KEY"); ok {
		datadogSite := getEnvOrDefault("DD_SITE", "datadoghq.com")
		datadogHostname := getEnvOrDefault("DD_HOSTNAME", "localhost")
		writers = append(writers, energy.NewDatadogWriter(datadogApiKey, datadogSite, datadogHostname, logger))
	} else {
		logger.Println("Skipping Datadog; DD_API_KEY not set")
	}

	tickLive := time.NewTicker(time.Second * time.Duration(10))
	tickPeriodic := time.NewTicker(time.Second * time.Duration(300))

	getMeterData(logger, reader, writers, LIVE|PERIODIC)
	go func() {
		for {
			select {
			case <-tickLive.C:
				getMeterData(logger, reader, writers, LIVE)
			case <-tickPeriodic.C:
				getMeterData(logger, reader, writers, PERIODIC)
			}
		}
	}()

	// Wait for a SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
