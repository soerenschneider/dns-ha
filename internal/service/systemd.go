package service

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Systemd struct {
	serviceName string
}

func NewSystemdService(serviceName string) (*Systemd, error) {
	if serviceName == "" {
		return nil, errors.New("empty service name provided")
	}

	exists, err := serviceExists(serviceName)
	if err != nil || !exists {
		return nil, fmt.Errorf("systemd service %q does not seem to exist", serviceName)
	}

	return &Systemd{serviceName: serviceName}, nil
}

func serviceExists(serviceName string) (bool, error) {
	cmd := exec.Command("systemctl", "status", serviceName)
	output, err := cmd.CombinedOutput()

	// If the output contains "Loaded: not-found", the service doesn't exist
	if strings.Contains(string(output), "Loaded: not-found") {
		return false, nil
	}

	if err != nil && errors.Is(err, exec.ErrNotFound) {
		return false, nil
	}

	return true, nil
}

func (s *Systemd) Reload() error {
	return reloadOrRestart("reload", s.serviceName)
}

func (s *Systemd) Restart() error {
	return reloadOrRestart("restart", s.serviceName)
}

func reloadOrRestart(operation string, serviceName string) error {
	cmd := exec.Command("systemctl", operation, serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to %s service %s: %w", operation, serviceName, err)
	}
	return nil
}
