package internal

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/soerenschneider/dns-ha/internal/conf"
	"github.com/soerenschneider/dns-ha/internal/metrics"
	"github.com/soerenschneider/dns-ha/internal/status"
)

var (
	ErrReloadNotSupported error = errors.New("reload not supported")
	PriorityComparator          = func(a, b ManagedDnsRecord) int {
		return cmp.Compare(b.Priority, a.Priority)
	}
)

type Healthcheck interface {
	IsHealthy(ctx context.Context) (bool, error)
}

type DnsRecord struct {
	Priority uint8
	DnsType  string
	Ip       net.IP
	Ttl      uint16
}

func NewDnsRecord(conf conf.RecordConfig) (DnsRecord, error) {
	parsed := net.ParseIP(conf.IP)
	if parsed == nil {
		return DnsRecord{}, fmt.Errorf("could not parse %s as ip address", conf.IP)
	}

	if parsed.To4() == nil && conf.RecordType != "AAAA" {
		return DnsRecord{}, fmt.Errorf("refusing to use IPv4 address with AAAA record")
	}

	return DnsRecord{
		Priority: uint8(conf.Prio), //nolint G115
		DnsType:  conf.RecordType,
		Ip:       parsed,
		Ttl:      uint16(conf.Ttl), //nolint G115
	}, nil

}

type ManagedDnsRecord struct {
	DnsRecord
	Hostname         string
	status           status.State
	healthCheck      Healthcheck
	lastStatusChange time.Time
}

func NewManagedDnsRecord(hostname string, record DnsRecord, statusOpts conf.StatusConfig, healthCheck Healthcheck) (*ManagedDnsRecord, error) {
	return &ManagedDnsRecord{
		Hostname:         hostname,
		DnsRecord:        record,
		status:           status.NewUnknownState(statusOpts),
		healthCheck:      healthCheck,
		lastStatusChange: time.Time{},
	}, nil
}

func (r *ManagedDnsRecord) GetState() status.State {
	return r.status
}

func (r *ManagedDnsRecord) Eval(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	isHealthy, err := r.healthCheck.IsHealthy(ctx)
	if err != nil {
		slog.Error("healthcheck produced error", "err", err)
		r.status.Error(r)
		return
	}

	slog.Debug("healthcheck", "healthy", isHealthy, "ip", r.Ip)
	if isHealthy {
		r.status.Healthy(r)
	} else {
		r.status.Unhealthy(r)
	}
}

func (r *ManagedDnsRecord) SetState(newStatus status.State) {
	// update metrics
	metrics.StatusChangeTimestamp.WithLabelValues(r.Hostname, r.Ip.String()).SetToCurrentTime()
	for _, state := range []string{status.HealthyStateName, status.UnhealthyStateName} {
		var val float64 = 0
		if newStatus.Name() == state {
			val = 1
		}
		metrics.Status.WithLabelValues(r.Hostname, r.Ip.String(), state).Set(val)
	}

	slog.Info("Status change", "record", r.Ip, "old", r.status.Name(), "new", newStatus.Name())
	r.status = newStatus
	r.lastStatusChange = time.Now()
}
