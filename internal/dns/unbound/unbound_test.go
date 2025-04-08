package unbound

import (
	"context"
	"reflect"
	"testing"

	"github.com/soerenschneider/dns-ha/internal"
	"github.com/soerenschneider/dns-ha/internal/conf"

	"log"
)

type dummyUnboundFs struct {
	read     []string
	readErr  error
	written  []string
	writeErr error
}

func (d *dummyUnboundFs) ReadConf() ([]string, error) {
	return d.read, d.readErr
}

func (d *dummyUnboundFs) ValidateConfig(_ context.Context) error {
	return nil
}

func (d *dummyUnboundFs) WriteConf(conf []string) error {
	d.written = conf
	return d.writeErr
}

type dummyHealthCheck struct{}

func (d *dummyHealthCheck) IsHealthy(_ context.Context) (bool, error) {
	return true, nil
}

func mustNewDnsRecord(conf conf.RecordConfig, healthCheck internal.Healthcheck) internal.ManagedDnsRecord {
	record, err := internal.NewDnsRecord(conf)
	if err != nil {
		log.Fatal(err)
	}
	managed, err := internal.NewManagedDnsRecord("hostname", record, conf.StatusConfig, healthCheck)
	if err != nil {
		log.Fatal(err)
	}

	return *managed
}

func TestUnbound_UpdateIps(t *testing.T) {
	type fields struct {
		fs UnboundConfWrapper
	}
	type args struct {
		dnsRecord string
		records   []internal.ManagedDnsRecord
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        bool
		wantWritten []string
		wantErr     bool
	}{
		{
			name: "record exists, no update needed",
			fields: fields{
				fs: &dummyUnboundFs{
					read: []string{
						`local-data: "test-01.my.tld 30 A 192.168.1.5"`,
						`local-data: "test-01.other.tld 30 A 192.168.1.5"`,
					},
					readErr:  nil,
					written:  nil,
					writeErr: nil,
				},
			},
			args: args{
				dnsRecord: "test-01.my.tld",
				records: []internal.ManagedDnsRecord{
					mustNewDnsRecord(conf.RecordConfig{
						IP:         "192.168.1.5",
						RecordType: "A",
						Prio:       200,
						Ttl:        30,
					}, &dummyHealthCheck{}),
				},
			},
			want:        false,
			wantWritten: nil,
			wantErr:     false,
		},
		{
			name: "A and AAAA records exist, no update needed",
			fields: fields{
				fs: &dummyUnboundFs{
					read: []string{
						`local-data: "test-01.my.tld 30 A 192.168.1.1"`,
						`local-data: "test-01.my.tld 30 AAAA ::1"`,
					},
					readErr:  nil,
					written:  nil,
					writeErr: nil,
				},
			},
			args: args{
				dnsRecord: "test-01.my.tld",
				records: []internal.ManagedDnsRecord{
					mustNewDnsRecord(conf.RecordConfig{
						IP:         "192.168.1.1",
						RecordType: "A",
						Prio:       0,
						Ttl:        30,
					}, &dummyHealthCheck{}),
					mustNewDnsRecord(conf.RecordConfig{
						IP:         "::1",
						RecordType: "AAAA",
						Prio:       0,
						Ttl:        30,
					}, &dummyHealthCheck{}),
				},
			},
			want:        false,
			wantWritten: nil,
			wantErr:     false,
		},
		{
			name: "record exists but ttl differs",
			fields: fields{
				fs: &dummyUnboundFs{
					read: []string{
						`local-data: "test-01.my.tld 60 A 192.168.1.5"`,
						`local-data: "test-01.other.tld 30 A 192.168.1.5"`,
					},
					readErr:  nil,
					written:  nil,
					writeErr: nil,
				},
			},
			args: args{
				dnsRecord: "test-01.my.tld",
				records: []internal.ManagedDnsRecord{
					mustNewDnsRecord(conf.RecordConfig{
						IP:         "192.168.1.5",
						RecordType: "A",
						Prio:       200,
						Ttl:        30,
					}, &dummyHealthCheck{}),
				},
			},
			want: true,
			wantWritten: []string{
				`local-data: "test-01.other.tld 30 A 192.168.1.5"`,
				`local-data: "test-01.my.tld 30 A 192.168.1.5"`,
			},
			wantErr: false,
		},
		{
			name: "record exists but IP differs",
			fields: fields{
				fs: &dummyUnboundFs{
					read: []string{
						`local-data: "test-01.my.tld 60 A 192.168.1.25"`,
						`local-data: "test-01.other.tld 30 A 192.168.1.5"`,
					},
					readErr:  nil,
					written:  nil,
					writeErr: nil,
				},
			},
			args: args{
				dnsRecord: "test-01.my.tld",
				records: []internal.ManagedDnsRecord{
					mustNewDnsRecord(conf.RecordConfig{
						IP:         "192.168.1.5",
						RecordType: "A",
						Prio:       200,
						Ttl:        30,
					}, &dummyHealthCheck{}),
				},
			},
			want: true,
			wantWritten: []string{
				`local-data: "test-01.other.tld 30 A 192.168.1.5"`,
				`local-data: "test-01.my.tld 30 A 192.168.1.5"`,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Unbound{
				fs: tt.fields.fs,
			}
			got, err := u.UpdateIps(tt.args.dnsRecord, tt.args.records)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateIps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("UpdateIps() got = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(tt.fields.fs.(*dummyUnboundFs).written, tt.wantWritten) {
				t.Errorf("UpdateIps() got written = %v, want %v", tt.fields.fs.(*dummyUnboundFs).written, tt.wantWritten)
			}
		})
	}
}
