package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/simonwhitaker/geo-energy-datadog/energy"
)

func TestReadinessHandlerReturnsUnavailableBeforeInitialSuccess(t *testing.T) {
	health := &healthState{}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	health.readinessHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestReadinessHandlerReturnsOKAfterRecentSuccess(t *testing.T) {
	health := &healthState{}
	health.markSuccess(LIVE | PERIODIC)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	health.readinessHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestReadinessHandlerReturnsUnavailableWhenLiveReadingsAreStale(t *testing.T) {
	now := time.Now()
	health := &healthState{}
	health.ready.Store(true)
	health.lastLiveOK.Store(now.Add(-liveReadinessTimeout - time.Second).Unix())
	health.lastPeriodicOK.Store(now.Unix())
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	health.readinessHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestReadinessHandlerReturnsUnavailableWhenPeriodicReadingsAreStale(t *testing.T) {
	now := time.Now()
	health := &healthState{}
	health.ready.Store(true)
	health.lastLiveOK.Store(now.Unix())
	health.lastPeriodicOK.Store(now.Add(-periodicReadinessTimeout - time.Second).Unix())
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	health.readinessHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

type fakeReader struct {
	live     []energy.Reading
	meter    []energy.Reading
	err      error
	panicMsg string
	calls    int
}

func (f *fakeReader) readings(r []energy.Reading) ([]energy.Reading, error) {
	f.calls++
	if f.panicMsg != "" {
		panic(f.panicMsg)
	}
	return r, f.err
}

func (f *fakeReader) GetLiveReadings() ([]energy.Reading, error)  { return f.readings(f.live) }
func (f *fakeReader) GetMeterReadings() ([]energy.Reading, error) { return f.readings(f.meter) }

type fakeWriter struct {
	written []energy.Reading
	err     error
}

func (w *fakeWriter) WriteReadings(r []energy.Reading) error {
	w.written = append(w.written, r...)
	return w.err
}

func (w *fakeWriter) Close() error { return nil }

func newTestPoller(reader energy.EnergyDataReader, writers ...energy.EnergyDataWriter) *poller {
	logger := log.New(io.Discard, "", 0)
	return newPoller(logger, &healthState{}, reader, writers)
}

func TestPollTurnsAPanicIntoAnError(t *testing.T) {
	p := newTestPoller(&fakeReader{panicMsg: "runtime error: invalid memory address or nil pointer dereference"})

	err := p.poll(LIVE)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "panic while reading live data") {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.health.ready.Load() {
		t.Fatal("expected the app to stay unready after a panic")
	}
}

func TestPollDoesNotMarkSuccessWhenUpstreamHasNoReadings(t *testing.T) {
	p := newTestPoller(&fakeReader{live: []energy.Reading{}})

	if err := p.poll(LIVE); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.health.ready.Load() {
		t.Fatal("expected the app to stay unready when no readings are available")
	}
}

func TestPollMarksSuccessWhenReadingsArrive(t *testing.T) {
	reader := &fakeReader{live: []energy.Reading{{Commodity: energy.ELECTRICITY, ReadingType: energy.LIVE, Value: 420}}}
	writer := &fakeWriter{}
	p := newTestPoller(reader, writer)

	if err := p.poll(LIVE); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !p.health.ready.Load() {
		t.Fatal("expected the app to be ready after a successful read")
	}
	if len(writer.written) != 1 {
		t.Fatalf("expected 1 reading to be written, got %d", len(writer.written))
	}
}

func TestGetMeterDataWritesToEveryWriterEvenWhenOneFails(t *testing.T) {
	reader := &fakeReader{live: []energy.Reading{{Commodity: energy.GAS, ReadingType: energy.LIVE, Value: 1448}}}
	failing := &fakeWriter{err: errors.New("exporter down")}
	working := &fakeWriter{}

	count, err := getMeterData(reader, []energy.EnergyDataWriter{failing, working}, LIVE)

	if err == nil {
		t.Fatal("expected the writer error to be reported")
	}
	if count != 1 {
		t.Fatalf("expected 1 reading, got %d", count)
	}
	if len(working.written) != 1 {
		t.Fatal("expected the healthy writer to still receive the reading")
	}
}

func TestWaitForConnectionGivesUpWhenContextIsCancelled(t *testing.T) {
	p := newTestPoller(&fakeReader{err: errors.New("Response Code: 502")})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if waitForConnection(ctx, log.New(io.Discard, "", 0), p) {
		t.Fatal("expected waitForConnection to report failure on a cancelled context")
	}
}
