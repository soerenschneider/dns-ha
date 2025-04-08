package conf

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v3"
)

const (
	defaultUnboundServiceName = "unbound"
	defaultMetricsAddr        = "127.0.0.1:9223"
)

var (
	validate *validator.Validate = validator.New()
)

type Config struct {
	Records map[string][]RecordConfig `json:"records" yaml:"records" validate:"dive,dive"`
	Unbound UnboundConfig             `json:"unbound" yaml:"unbound"`

	MetricsFile string `json:"metrics_file" yaml:"metrics_file" validate:"excluded_with=MetricsAddr,omitempty,filepath"`
	MetricsAddr string `json:"metrics_addr" yaml:"metrics_addr" validate:"excluded_with=MetricsFile,omitempty,hostname_port"`
}

func (c *Config) Validate() error {
	var errs error
	if err := validate.Struct(c); err != nil {
		errs = multierr.Append(errs, err)
	}

	for record, ips := range c.Records {
		if err := validate.Var(record, "required,hostname"); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("%q is not a valid hostname", record))
		}

		if len(ips) < 2 {
			errs = multierr.Append(errs, fmt.Errorf("less than two records defined for %q", record))
		}

		seenPrios := make(map[int]struct{}, len(ips))
		seenIps := make(map[string]struct{}, len(ips))
		for _, ip := range ips {
			_, found := seenPrios[ip.Prio]
			seenPrios[ip.Prio] = struct{}{}
			if found {
				errs = multierr.Append(errs, fmt.Errorf("duplicated prio %d for record %s", ip.Prio, record))
			}

			_, found = seenIps[ip.IP]
			seenIps[ip.IP] = struct{}{}
			if found {
				errs = multierr.Append(errs, fmt.Errorf("duplicated ip %s for record %s", ip.IP, record))
			}
		}
	}

	return errs
}

type RecordConfig struct {
	IP         string `json:"ip" yaml:"ip" validate:"required,ip"`
	RecordType string `json:"type" yaml:"type" validate:"required,oneof=A AAAA"`
	Prio       int    `json:"prio" yaml:"prio" validate:"required,gte=0,lt=255"`
	Ttl        int    `json:"ttl" yaml:"ttl" validate:"gte=1,lte=3600"`

	HealthcheckConfig map[string]any `json:"healthchecker" yaml:"healthchecker" validate:"required"`
	StatusConfig      StatusConfig   `json:"status" yaml:"status"`
}

func (conf *RecordConfig) UnmarshalYAML(node *yaml.Node) error {
	type Alias RecordConfig // Create an alias to avoid recursion during unmarshalling

	// Define conf temporary struct with default values
	tmp := &Alias{
		StatusConfig: StatusConfig{
			HealthyStreak:          5,
			UnhealthyStreak:        5,
			InitialHealthyStreak:   2,
			InitialUnhealthyStreak: 1,
		},
	}

	// Unmarshal the yaml data into the temporary struct
	if err := node.Decode(&tmp); err != nil {
		return err
	}

	// Assign the values from the temporary struct to the original struct
	*conf = RecordConfig(*tmp)
	return nil
}

type StatusConfig struct {
	HealthyStreak          int `yaml:"healthy" validate:"gte=1"`
	UnhealthyStreak        int `yaml:"unhealthy" validate:"gte=1"`
	InitialHealthyStreak   int `yaml:"initial_healthy" validate:"gte=1"`
	InitialUnhealthyStreak int `yaml:"initial_unhealthy" validate:"gte=1"`
}

type UnboundConfig struct {
	DbFile      string `json:"db_file" yaml:"db_file" validate:"filepath"`
	ServiceName string `json:"service_name" yaml:"service_name"`
	CreateFile  bool   `json:"create_file" yaml:"create_file"`
}

func ReadFromFile(filePath string) (*Config, error) {
	conf := Config{
		MetricsAddr: defaultMetricsAddr,
		Unbound: UnboundConfig{
			ServiceName: defaultUnboundServiceName,
			CreateFile:  true,
		},
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
