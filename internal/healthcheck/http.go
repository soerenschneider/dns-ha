package healthcheck

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/soerenschneider/dns-ha/internal"
)

const (
	HttpCheckerName = "http"
	defaultMethod   = http.MethodGet
)

var defaultStatusCodes = []int{200, 201, 301}

type Http struct {
	endpoint          string
	method            string
	wantedStatusCodes []int
	httpClient        *http.Client
}

func NewHttp(host string, record internal.DnsRecord, args map[string]any) (*Http, error) {
	if host == "" {
		return nil, errors.New("empty endpoint supplied")
	}

	var method = defaultMethod
	var statusCodes = defaultStatusCodes

	var httpClient *http.Client
	var endpoint string

	usesTls := false
	usesTlsRaw, found := args["use_tls"]
	if found {
		var err error
		usesTls, err = strconv.ParseBool(usesTlsRaw.(string))
		if err != nil {
			return nil, errors.New("could not parse use_tls as bool from map")
		}
	}
	if usesTls {
		httpClient = newHTTPClientWithHost(host)
		endpoint = "https://" + record.Ip.String()
	} else {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
		}
		endpoint = "http://" + record.Ip.String()
	}

	portRaw, found := args["port"]
	if found {
		port, err := strconv.Atoi(portRaw.(string))
		if err != nil {
			return nil, errors.New("could not parse port as integer")
		}
		endpoint += fmt.Sprintf(":%d", port)
	}

	return &Http{
		endpoint:          endpoint,
		method:            method,
		wantedStatusCodes: statusCodes,
		httpClient:        httpClient,
	}, nil
}

func newHTTPClientWithHost(host string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS13,
		ServerName: host,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
}

func (h *Http) IsHealthy(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, h.method, h.endpoint, nil)
	if err != nil {
		return false, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()
	return slices.Contains(h.wantedStatusCodes, resp.StatusCode), nil
}
