package healthcheck

import (
	"cmp"
	"context"
	"fmt"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/soerenschneider/dns-ha/internal"

	"runtime"
	"time"
)

const (
	IcmpCheckerName    = "icmp"
	icmpDefaultTimeout = 3 * time.Second
)

type IcmpChecker struct {
	host       string
	timeout    time.Duration
	privileged bool
}

func NewIcmpChecker(record internal.DnsRecord, args map[string]any) (*IcmpChecker, error) {
	ret := &IcmpChecker{
		host:       record.Ip.String(),
		timeout:    icmpDefaultTimeout,
		privileged: getPrivilegedDefaultForPlatform(),
	}

	timeoutHuman, ok := args["timeout"].(string)
	if ok {
		timeout, err := time.ParseDuration(timeoutHuman)
		if err != nil {
			return nil, fmt.Errorf("timeout duration could not be parsed: %w", err)
		}
		ret.timeout = timeout
	}

	privileged, ok := args["privileged"].(bool)
	if ok {
		ret.privileged = privileged
	}

	return ret, nil
}

func getPrivilegedDefaultForPlatform() bool {
	switch runtime.GOOS {
	case "linux":
		return true
	case "windows":
		return true
	}

	return false
}

func (c *IcmpChecker) IsHealthy(ctx context.Context) (bool, error) {
	pinger, err := probing.NewPinger(c.host)
	if err != nil {
		return false, fmt.Errorf("could not create pinger: %w", err)
	}

	count := 1
	pinger.Timeout = cmp.Or(c.timeout, defaultTimeout)
	pinger.Count = count
	pinger.SetPrivileged(c.privileged)
	if err := pinger.RunWithContext(ctx); err != nil {
		return false, fmt.Errorf("ping unsuccessful: %w", err)
	}

	stats := pinger.Statistics()
	return stats.PacketsRecv == count, nil
}
