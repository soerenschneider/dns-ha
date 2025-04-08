package healthcheck

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/soerenschneider/dns-ha/internal"
)

const (
	TcpCheckerName = "tcp"
	defaultTimeout = 5 * time.Second
)

type TcpChecker struct {
	host    string
	port    string
	timeout time.Duration
}

func NewTcpChecker(record internal.DnsRecord, args map[string]any) (*TcpChecker, error) {
	portRaw, found := args["port"]
	if !found {
		return nil, errors.New("missing port in args")
	}

	_, err := strconv.Atoi(portRaw.(string))
	if err != nil {
		return nil, errors.New("could not parse port as integer")
	}

	ret := &TcpChecker{
		host: record.Ip.String(),
		port: portRaw.(string),
	}

	timeoutHuman, ok := args["timeout"].(string)
	if ok {
		timeout, err := time.ParseDuration(timeoutHuman)
		if err != nil {
			return nil, fmt.Errorf("timeout duration could not be parsed: %w", err)
		}
		ret.timeout = timeout
	}

	return ret, nil
}

func (c *TcpChecker) IsHealthy(_ context.Context) (bool, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(c.host, c.port), cmp.Or(c.timeout, defaultTimeout))
	if err == nil && conn != nil {
		defer conn.Close()
		return true, nil
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		// receiving this error means the remote system replied
		return true, nil
	}

	return false, err
}
