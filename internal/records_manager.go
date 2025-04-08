package internal

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"

	"github.com/soerenschneider/dns-ha/internal/metrics"
	"github.com/soerenschneider/dns-ha/internal/status"
)

type DnsDb interface {
	UpdateIps(dnsRecord string, addresses []ManagedDnsRecord) (bool, error)
	ValidateConfig(ctx context.Context) error
}

type Service interface {
	Reload() error
	Restart() error
}

type RecordManager struct {
	dnsDb          DnsDb
	dnsServiceUnit Service
	managedRecords map[string][]*ManagedDnsRecord

	unhealthyHosts map[string]bool
}

func NewRecordManager(dnsDb DnsDb, dnsService Service, managedRecords map[string][]*ManagedDnsRecord) (*RecordManager, error) {
	return &RecordManager{
		dnsDb:          dnsDb,
		dnsServiceUnit: dnsService,
		managedRecords: managedRecords,
		unhealthyHosts: make(map[string]bool, len(managedRecords)),
	}, nil
}

func (h *RecordManager) CheckRecords(ctx context.Context) {
	h.runHealthchecks(ctx)

	restartServiceNeeded := false
	for hostname, ips := range h.managedRecords {
		if h.updateRecords(ctx, hostname, ips) {
			restartServiceNeeded = true
		}
	}

	if restartServiceNeeded {
		if err := h.restartService(); err != nil {
			metrics.Errors.WithLabelValues("", "service_restart").Inc()
			slog.Error("could not restart service")
		}
	}
}

func (h *RecordManager) updateRecords(ctx context.Context, hostname string, ips []*ManagedDnsRecord) bool {
	ipsToUpdate := filterHealthyIps(hostname, ips)
	if len(ipsToUpdate) == 0 {
		if !h.unhealthyHosts[hostname] && !isInitialState(ips) {
			slog.Warn("No healthy IPs detected", "hostname", hostname)
			h.unhealthyHosts[hostname] = true
		}
		return false
	}

	if h.unhealthyHosts[hostname] {
		slog.Info("Records for hostname recovered from unhealthy state", "hostname", hostname)
		h.unhealthyHosts[hostname] = false
	}

	updated, err := h.dnsDb.UpdateIps(hostname, ipsToUpdate)
	if err != nil {
		metrics.Errors.WithLabelValues(hostname, "update_ips").Inc()
		slog.Error("could not update active IPs", "hostname", hostname)
		return false
	}

	if updated {
		ipsToUpdateLog := make([]string, len(ipsToUpdate))
		for index, ip := range ipsToUpdate {
			ipsToUpdateLog[index] = ip.Ip.String()
		}
		slog.Info("Updating DNS records", "hostname", hostname, "ips", ipsToUpdateLog)

		if err := h.dnsDb.ValidateConfig(ctx); err != nil {
			metrics.Errors.WithLabelValues(hostname, "dns_invalid_config").Inc()
			slog.Error("updated unbound config produced error", "err", err)
		} else {
			return true
		}
	}

	return false
}

func (h *RecordManager) runHealthchecks(ctx context.Context) {
	wg := &sync.WaitGroup{}
	for _, candidates := range h.managedRecords {
		for _, candidate := range candidates {
			wg.Add(1)
			go candidate.Eval(ctx, wg)
		}
	}

	wg.Wait()
}

func isInitialState(ips []*ManagedDnsRecord) bool {
	for _, ip := range ips {
		if ip.GetState().Name() != status.InitialStateName {
			return false
		}
	}

	return true
}

func filterHealthyIps(hostname string, ips []*ManagedDnsRecord) []ManagedDnsRecord {
	healthyIps := make(map[string][]ManagedDnsRecord, len(ips))
	for _, ip := range ips {
		if ip.GetState().Name() == status.HealthyStateName {
			_, found := healthyIps[ip.DnsType]
			if !found {
				healthyIps[ip.DnsType] = []ManagedDnsRecord{}
			}
			healthyIps[ip.DnsType] = append(healthyIps[ip.DnsType], *ip)
		}
	}

	activeIps := make(map[string]bool, len(ips))
	defer updateMetrics(hostname, ips, activeIps)
	if len(healthyIps) == 0 {
		return nil
	}

	ipsToUpdate := make([]ManagedDnsRecord, 0, len(ips))
	for _, healthyRecordsByDnsType := range healthyIps {
		if len(healthyRecordsByDnsType) > 0 {
			slices.SortFunc(healthyRecordsByDnsType, PriorityComparator)
			ipsToUpdate = append(ipsToUpdate, healthyRecordsByDnsType[0])
			activeIps[healthyRecordsByDnsType[0].Ip.String()] = true
		}
	}

	return ipsToUpdate
}

func updateMetrics(hostname string, ips []*ManagedDnsRecord, activeIps map[string]bool) {
	cntActive := 0

	for _, ip := range ips {
		_, found := activeIps[ip.Ip.String()]
		if found {
			cntActive++
			metrics.ActiveRecord.WithLabelValues(hostname, ip.Ip.String()).Set(1)
		} else {
			metrics.ActiveRecord.WithLabelValues(hostname, ip.Ip.String()).Set(0)
		}
	}

	metrics.ActiveRecords.WithLabelValues(hostname).Set(float64(cntActive))
	metrics.ConfiguredRecords.WithLabelValues(hostname).Set(float64(len(ips)))
}

func (h *RecordManager) restartService() error {
	if err := h.dnsServiceUnit.Reload(); err != nil {
		if !errors.Is(err, ErrReloadNotSupported) {
			slog.Error("could not reload service", "err", err)
		}

		if err := h.dnsServiceUnit.Restart(); err != nil {
			return err
		}
	}

	return nil
}
