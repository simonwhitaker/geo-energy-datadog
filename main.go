package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/simonwhitaker/geo-energy-datadog/energy"
)

type ReadingMode int

const (
	LIVE ReadingMode = 1 << iota
	PERIODIC
)

func (m ReadingMode) String() string {
	switch m {
	case LIVE:
		return "live"
	case PERIODIC:
		return "periodic"
	case LIVE | PERIODIC:
		return "live and periodic"
	default:
		return "unknown"
	}
}

const (
	healthServerAddr         = ":8080"
	liveReadinessTimeout     = time.Minute
	periodicReadinessTimeout = 15 * time.Minute
)

type healthState struct {
	ready          atomic.Bool
	lastLiveOK     atomic.Int64
	lastPeriodicOK atomic.Int64
}

func (h *healthState) markSuccess(mode ReadingMode) {
	now := time.Now().Unix()

	if mode&LIVE != 0 {
		h.lastLiveOK.Store(now)
	}
	if mode&PERIODIC != 0 {
		h.lastPeriodicOK.Store(now)
	}

	h.ready.Store(true)
}

func (h *healthState) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *healthState) readinessHandler(w http.ResponseWriter, _ *http.Request) {
	if !h.ready.Load() {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}

	now := time.Now()
	lastLiveOK := time.Unix(h.lastLiveOK.Load(), 0)
	lastPeriodicOK := time.Unix(h.lastPeriodicOK.Load(), 0)

	if now.Sub(lastLiveOK) > liveReadinessTimeout {
		http.Error(w, "live readings stale", http.StatusServiceUnavailable)
		return
	}
	if now.Sub(lastPeriodicOK) > periodicReadinessTimeout {
		http.Error(w, "periodic readings stale", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func startHealthServer(logger *log.Logger, health *healthState) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.livenessHandler)
	mux.HandleFunc("/readyz", health.readinessHandler)

	server := &http.Server{
		Addr:    healthServerAddr,
		Handler: mux,
	}

	go func() {
		logger.Printf("Health server listening on %s", healthServerAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Health server failed: %v", err)
		}
	}()

	return server
}

// getMeterData reads the requested data and hands it to every writer. It
// returns the number of readings the upstream API had available.
func getMeterData(reader energy.EnergyDataReader, writers []energy.EnergyDataWriter, mode ReadingMode) (int, error) {
	allReadings := []energy.Reading{}

	if mode&PERIODIC != 0 {
		// Get periodic meter data
		readings, err := reader.GetMeterReadings()
		if err != nil {
			return 0, err
		}
		allReadings = append(allReadings, readings...)
	}
	if mode&LIVE != 0 {
		// Get live meter data
		readings, err := reader.GetLiveReadings()
		if err != nil {
			return 0, err
		}

		allReadings = append(allReadings, readings...)
	}

	// Write to every writer even if one of them fails; a broken exporter
	// shouldn't cost us the readings the others could have taken.
	var writeErrs []error
	for _, w := range writers {
		if err := w.WriteReadings(allReadings); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return len(allReadings), errors.Join(writeErrs...)
}

// poller reads from the meter on demand and keeps the health state in step
// with whether data is actually flowing.
type poller struct {
	logger     *log.Logger
	health     *healthState
	reader     energy.EnergyDataReader
	writers    []energy.EnergyDataWriter
	silentFrom map[ReadingMode]time.Time
}

func newPoller(logger *log.Logger, health *healthState, reader energy.EnergyDataReader, writers []energy.EnergyDataWriter) *poller {
	return &poller{
		logger:     logger,
		health:     health,
		reader:     reader,
		writers:    writers,
		silentFrom: map[ReadingMode]time.Time{},
	}
}

// poll fetches and writes one round of readings. A panic below this point (the
// upstream client dereferences a nil response when the transport fails) is
// turned into an error rather than being allowed to kill the process.
func (p *poller) poll(mode ReadingMode) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while reading %v data: %v", mode, r)
		}
	}()

	count, err := getMeterData(p.reader, p.writers, mode)
	if err != nil {
		return err
	}

	if count == 0 {
		// The API answers 200 with nothing in it while it's degraded. Don't
		// report that as a success: let readiness go stale instead of
		// claiming health while no data is flowing.
		p.noteSilence(mode)
		return nil
	}

	p.noteReadings(mode)
	p.health.markSuccess(mode)
	return nil
}

// noteSilence logs the start of a run of empty responses, once per run.
func (p *poller) noteSilence(mode ReadingMode) {
	if _, alreadySilent := p.silentFrom[mode]; alreadySilent {
		return
	}

	p.silentFrom[mode] = time.Now()
	p.logger.Printf("No %v readings available from upstream", mode)
}

func (p *poller) noteReadings(mode ReadingMode) {
	if since, wasSilent := p.silentFrom[mode]; wasSilent {
		p.logger.Printf("%v readings resumed after %v", mode, time.Since(since).Truncate(time.Second))
		delete(p.silentFrom, mode)
	}
}

// waitForConnection retries the first read with exponential backoff until it
// succeeds, or until ctx is cancelled. It reports whether it connected.
func waitForConnection(ctx context.Context, logger *log.Logger, p *poller) bool {
	backoff := time.Second * 5
	maxBackoff := time.Minute * 2

	for {
		err := p.poll(LIVE | PERIODIC)
		if err == nil {
			logger.Println("Connected successfully")
			return true
		}

		logger.Printf("Connection failed: %v (retrying in %v)", err, backoff)

		// Stay responsive to shutdown while we're waiting to retry.
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}

		// Exponential backoff with cap
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	health := &healthState{}

	// Configure reader
	geoUsername := os.Getenv("GEO_USERNAME")
	geoPassword := os.Getenv("GEO_PASSWORD")
	reader := energy.NewGeoEnergyDataReader(geoUsername, geoPassword)

	// Configure writers
	writers := []energy.EnergyDataWriter{
		energy.NewLoggerWriter(logger),
	}

	if _, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT"); ok {
		otelHostname := getEnvOrDefault("OTEL_HOSTNAME", "localhost")
		otelWriter, err := energy.NewOTelWriter(context.Background(), otelHostname, logger)
		if err != nil {
			logger.Fatalf("Failed to initialize OTel writer: %v", err)
		}
		writers = append(writers, otelWriter)
	} else {
		logger.Println("Skipping OTel; OTEL_EXPORTER_OTLP_ENDPOINT not set")
	}

	healthServer := startHealthServer(logger, health)

	// Shut down cleanly on SIGINT or SIGTERM, including while we're still
	// retrying the initial connection.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	p := newPoller(logger, health, reader, writers)

	// Wait for initial connection with retry
	if waitForConnection(ctx, logger, p) {
		tickLive := time.NewTicker(time.Second * time.Duration(10))
		tickPeriodic := time.NewTicker(time.Second * time.Duration(300))
		defer tickLive.Stop()
		defer tickPeriodic.Stop()

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-tickLive.C:
					if err := p.poll(LIVE); err != nil {
						logger.Printf("Error getting live data: %v", err)
					}
				case <-tickPeriodic.C:
					if err := p.poll(PERIODIC); err != nil {
						logger.Printf("Error getting periodic data: %v", err)
					}
				}
			}
		}()

		<-ctx.Done()
	}

	logger.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Printf("Error shutting down health server: %v", err)
	}
	for _, w := range writers {
		if err := w.Close(); err != nil {
			logger.Printf("Error closing writer: %v", err)
		}
	}
}
