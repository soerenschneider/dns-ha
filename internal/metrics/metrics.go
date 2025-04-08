package metrics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"go.uber.org/multierr"
)

const (
	namespace                        = "dns_ha"
	defaultMetricsHeartbeatFrequency = 1 * time.Minute
)

var (
	ProcessStart = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "process_start_timestamp_seconds",
		Help:      "Timestamp of start of process",
	})

	Heartbeat = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "heartbeat_timestamp_seconds",
		Help:      "Continuous heartbeat",
	})

	Errors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "errors_total",
		Help:      "Total amount of errors",
	}, []string{"hostname", "error"})

	Status = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "status",
		Help:        "Captures the current status",
		ConstLabels: nil,
	}, []string{"hostname", "ip", "status"})

	StatusChangeTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "status_change_timestamp_seconds",
		Help:        "Captures the current status",
		ConstLabels: nil,
	}, []string{"hostname", "ip"})

	ActiveRecord = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "active_record",
		Help:        "Captures the current status",
		ConstLabels: nil,
	}, []string{"hostname", "ip"})

	ActiveRecords = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "active_records_total",
		Help:        "Captures the current status",
		ConstLabels: nil,
	}, []string{"hostname"})

	ConfiguredRecords = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "configured_records_total",
		Help:        "Captures the current status",
		ConstLabels: nil,
	}, []string{"hostname"})
)

func init() {
	ProcessStart.SetToCurrentTime()
	Heartbeat.SetToCurrentTime()
}

type MetricsServer struct {
	address string
}

type MetricsServerOpts func(*MetricsServer) error

func New(address string, opts ...MetricsServerOpts) (*MetricsServer, error) {
	if len(address) == 0 {
		return nil, errors.New("empty address provided")
	}

	w := &MetricsServer{
		address: address,
	}

	var errs error
	for _, opt := range opts {
		if err := opt(w); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return w, errs
}

func (s *MetricsServer) StartServer(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := http.Server{
		Addr:              s.address,
		Handler:           mux,
		ReadTimeout:       1 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	errChan := make(chan error)
	go func() {
		slog.Info("Starting server", "address", s.address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("can not start metrics server: %w", err)
		}
	}()

	heartbeatTimer := time.NewTicker(defaultMetricsHeartbeatFrequency)
	defer heartbeatTimer.Stop()

	for {
		select {
		case <-heartbeatTimer.C:
			Heartbeat.SetToCurrentTime()
		case <-ctx.Done():
			slog.Info("Stopping server")
			return server.Shutdown(ctx)
		case err := <-errChan:
			return err
		}
	}
}

func StartMetricsWriter(ctx context.Context, wg *sync.WaitGroup, path string) {
	defer wg.Done()
	ticker := time.NewTicker(defaultMetricsHeartbeatFrequency)

	for {
		select {
		case <-ticker.C:
			Heartbeat.SetToCurrentTime()
			if err := WriteMetrics(path); err != nil {
				slog.Error("Error dumping metrics", "err", err)
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func WriteMetrics(metricsFile string) error {
	metrics, err := dumpMetrics()
	if err != nil {
		return err
	}

	tmpFile := fmt.Sprintf("%s.tmp", metricsFile)
	if err := os.WriteFile(tmpFile, []byte(metrics), 0644); err != nil { //nolint G306
		return fmt.Errorf("error creating file: %w", err)
	}
	return os.Rename(tmpFile, metricsFile)
}

func dumpMetrics() (string, error) {
	var buf = &bytes.Buffer{}
	fmt := expfmt.NewFormat(expfmt.TypeTextPlain)
	enc := expfmt.NewEncoder(buf, fmt)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return "", err
	}

	for _, f := range families {
		// Writing these metrics will cause a duplication error with other tools writing the same metrics
		if strings.HasPrefix(f.GetName(), namespace) {
			if err := enc.Encode(f); err != nil {
				slog.Warn("could not encode metric", "err", err.Error())
			}
		}
	}

	return buf.String(), nil
}
