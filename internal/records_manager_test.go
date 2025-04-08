package internal

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/soerenschneider/dns-ha/internal/status"
)

type dummyHealthcheck struct {
	ret    bool
	retErr error
}

func (d *dummyHealthcheck) IsHealthy(ctx context.Context) (bool, error) {
	return d.ret, d.retErr
}

func Test_getHealthyIps(t *testing.T) {
	type args struct {
		hostname string
		ips      []*ManagedDnsRecord
	}
	tests := []struct {
		name string
		args args
		want []ManagedDnsRecord
	}{
		{
			name: "",
			args: args{
				hostname: "xxx",
				ips: []*ManagedDnsRecord{
					{
						DnsRecord: DnsRecord{
							Priority: 250,
							DnsType:  "A",
							Ip:       net.ParseIP("192.168.1.1"),
							Ttl:      60,
						},
						Hostname:    "xxx",
						status:      &status.Healthy{},
						healthCheck: &dummyHealthcheck{},
					},
					{
						DnsRecord: DnsRecord{
							Priority: 250,
							DnsType:  "A",
							Ip:       net.ParseIP("192.168.1.2"),
							Ttl:      60,
						},
						Hostname:    "xxx",
						status:      &status.Unhealthy{},
						healthCheck: &dummyHealthcheck{},
					},
					{
						DnsRecord: DnsRecord{
							Priority: 250,
							DnsType:  "A",
							Ip:       net.ParseIP("192.168.1.2"),
							Ttl:      60,
						},
						Hostname:    "xxx",
						status:      &status.Unhealthy{},
						healthCheck: &dummyHealthcheck{},
					},
				},
			},
			want: []ManagedDnsRecord{{
				DnsRecord: DnsRecord{
					Priority: 250,
					DnsType:  "A",
					Ip:       net.ParseIP("192.168.1.1"),
					Ttl:      60,
				},
				Hostname:    "xxx",
				status:      &status.Healthy{},
				healthCheck: &dummyHealthcheck{},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterHealthyIps(tt.args.hostname, tt.args.ips); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterHealthyIps() = %v, want %v", got, tt.want)
			}
		})
	}
}
