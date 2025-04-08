package conf

import (
	"testing"
)

func TestConf_Validate(t *testing.T) {
	type fields struct {
		Records map[string][]RecordConfig
		Unbound UnboundConfig

		MetricsFile string
		MetricsAddr string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "invalid hostname",
			fields: fields{
				Unbound: UnboundConfig{
					DbFile:      "/path/to/file",
					ServiceName: "unbound",
				},
				Records: map[string][]RecordConfig{
					"not/a/valid/hostname": []RecordConfig{
						{
							IP:                "10.0.0.1",
							RecordType:        "A",
							Prio:              10,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
						{
							IP:                "10.0.0.2",
							RecordType:        "A",
							Prio:              10,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid config",
			fields: fields{
				MetricsFile: "127.0.0.1:666",
				Unbound: UnboundConfig{
					DbFile:      "path/to/file",
					ServiceName: "unbound",
				},
				Records: map[string][]RecordConfig{
					"my.tld": []RecordConfig{
						{
							IP:                "10.0.0.1",
							RecordType:        "A",
							Prio:              20,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
						{
							IP:                "10.0.0.2",
							RecordType:        "A",
							Prio:              10,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicated ip",
			fields: fields{
				Unbound: UnboundConfig{
					DbFile:      "path/to/file",
					ServiceName: "unbound",
				},
				Records: map[string][]RecordConfig{
					"my.tld": []RecordConfig{
						{
							IP:                "10.0.0.1",
							RecordType:        "A",
							Prio:              20,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
						{
							IP:                "10.0.0.1",
							RecordType:        "A",
							Prio:              10,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicated prio",
			fields: fields{
				Unbound: UnboundConfig{
					DbFile:      "path/to/file",
					ServiceName: "unbound",
				},
				Records: map[string][]RecordConfig{
					"my.tld": []RecordConfig{
						{
							IP:                "10.0.0.1",
							RecordType:        "A",
							Prio:              20,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
						{
							IP:                "10.0.0.2",
							RecordType:        "A",
							Prio:              20,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "only one record",
			fields: fields{
				Unbound: UnboundConfig{
					DbFile:      "path/to/file",
					ServiceName: "unbound",
				},
				Records: map[string][]RecordConfig{
					"my.tld": []RecordConfig{
						{
							IP:                "10.0.0.1",
							RecordType:        "A",
							Prio:              20,
							Ttl:               60,
							HealthcheckConfig: map[string]any{},
							StatusConfig: StatusConfig{
								HealthyStreak:          1,
								UnhealthyStreak:        1,
								InitialHealthyStreak:   1,
								InitialUnhealthyStreak: 1,
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Records:     tt.fields.Records,
				Unbound:     tt.fields.Unbound,
				MetricsAddr: tt.fields.MetricsAddr,
				MetricsFile: tt.fields.MetricsFile,
			}
			if err := c.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
