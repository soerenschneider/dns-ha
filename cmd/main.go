package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/soerenschneider/dns-ha/internal"
	"github.com/soerenschneider/dns-ha/internal/conf"
	"github.com/soerenschneider/dns-ha/internal/dns/unbound"
	"github.com/soerenschneider/dns-ha/internal/healthcheck"
	"github.com/soerenschneider/dns-ha/internal/metrics"
	"github.com/soerenschneider/dns-ha/internal/service"
	"go.uber.org/multierr"
)

const defaultConfigFile = "/etc/dns-ha.yaml"

var (
	flagConfigFile   string
	flagDebug        bool
	flagPrintVersion bool

	BuildVersion string
	CommitHash   string
)

func parseFlags() {
	flag.StringVar(&flagConfigFile, "config", defaultConfigFile, "Config file")
	flag.BoolVar(&flagDebug, "debug", false, "Print debug logs")
	flag.BoolVar(&flagPrintVersion, "version", false, "Print version and exit")
	flag.Parse()
}

func main() {
	parseFlags()

	if flagPrintVersion {
		//nolint forbidigo
		fmt.Printf("%s %s\n", BuildVersion, CommitHash)
		os.Exit(0)
	}

	setupLogging()
	slog.Info("Starting dns-ha", "version", BuildVersion)

	conf, err := conf.ReadFromFile(flagConfigFile)
	if err != nil {
		log.Fatalf("could not read config: %v", err)
	}

	if err := conf.Validate(); err != nil {
		log.Fatalf("validating config failed: %v", err)
	}

	dbConfWrapper, err := unbound.NewUnboundConfigWrapper(conf.Unbound.DbFile, conf.Unbound.CreateFile)
	if err != nil {
		log.Fatalf("could not create unbound config wrapper: %v", err)
	}
	var db internal.DnsDb
	db, err = unbound.NewUnbound(dbConfWrapper)
	if err != nil {
		log.Fatalf("could not create unbound service: %v", err)
	}

	var svc internal.Service
	svc, err = service.NewSystemdService("unbound")
	if err != nil {
		log.Fatalf("could not create systemd service: %v", err)
	}

	managedRecords, err := getManagedDnsRecords(conf.Records)
	if err != nil {
		log.Fatal(err)
	}
	run(db, svc, managedRecords, conf)
}

func run(db internal.DnsDb, svc internal.Service, managedRecords map[string][]*internal.ManagedDnsRecord, conf *conf.Config) {
	recordManager, err := internal.NewRecordManager(db, svc, managedRecords)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}
	metricsErrChan := make(chan error, 1)
	go func() {
		if conf.MetricsAddr != "" {
			wg.Add(1)
			metricsServer, err := metrics.New(conf.MetricsAddr)
			if err != nil {
				metricsErrChan <- err
			} else {
				if err := metricsServer.StartServer(ctx, wg); err != nil {
					metricsErrChan <- err
				}
			}
		} else if conf.MetricsFile != "" {
			wg.Add(1)
			metrics.StartMetricsWriter(ctx, wg, conf.MetricsFile)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		recordManager.CheckRecords(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				recordManager.CheckRecords(ctx)
			}
		}
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	var exitCode int
	select {
	case <-sigc:
		slog.Info("Received signal")
		exitCode = 0
	case err := <-metricsErrChan:
		slog.Error("could not start metrics subsystem", "err", err)
		exitCode = 1
	}

	cancel()
	gracefulExitDone := make(chan struct{})

	go func() {
		slog.Info("Waiting for components to shut down gracefully")
		wg.Wait()
		close(gracefulExitDone)
	}()

	select {
	case <-gracefulExitDone:
		slog.Debug("All components shut down gracefully within the timeout")
	case <-time.After(30 * time.Second):
		slog.Error("Killing process forcefully")
	}
	os.Exit(exitCode)
}

func getManagedDnsRecords(c map[string][]conf.RecordConfig) (map[string][]*internal.ManagedDnsRecord, error) {
	ret := make(map[string][]*internal.ManagedDnsRecord)
	var errs error

	for hostname, records := range c {
		var add []*internal.ManagedDnsRecord
		for _, recordConf := range records {
			record, err := internal.NewDnsRecord(recordConf)
			if err != nil {
				errs = multierr.Append(errs, fmt.Errorf("could not build record from config: %w", err))
			}

			healthchecker, err := buildHealthcheck(hostname, record, recordConf.HealthcheckConfig)
			if err != nil {
				errs = multierr.Append(errs, fmt.Errorf("could not build healthcheck: %w", err))
			}

			r, err := internal.NewManagedDnsRecord(hostname, record, recordConf.StatusConfig, healthchecker)
			if err != nil {
				errs = multierr.Append(errs, fmt.Errorf("could not build managed record: %w", err))
			}

			add = append(add, r)
		}
		ret[hostname] = add
	}

	return ret, errs
}

func setupLogging() {
	var level slog.Leveler = slog.LevelInfo
	if flagDebug {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func buildHealthcheck(host string, record internal.DnsRecord, args map[string]any) (internal.Healthcheck, error) {
	healthchecker, found := args["type"]
	if !found {
		return nil, errors.New("no type specified")
	}

	switch healthchecker {
	case healthcheck.HttpCheckerName:
		return healthcheck.NewHttp(host, record, args)
	case healthcheck.IcmpCheckerName:
		return healthcheck.NewIcmpChecker(record, args)
	case healthcheck.TcpCheckerName:
		return healthcheck.NewTcpChecker(record, args)
	default:
		return nil, fmt.Errorf("no checker %q available", healthchecker)
	}
}
