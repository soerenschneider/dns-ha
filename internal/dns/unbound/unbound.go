package unbound

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/soerenschneider/dns-ha/internal"
)

type Unbound struct {
	fs UnboundConfWrapper
}

// UnboundConfWrapper is just a simple wrapper to increase testability for Unbound.
type UnboundConfWrapper interface {
	ReadConf() ([]string, error)
	WriteConf(conf []string) error
	ValidateConfig(ctx context.Context) error
}

func NewUnbound(fs UnboundConfWrapper) (*Unbound, error) {
	if fs == nil {
		return nil, errors.New("nil fs supplied")
	}

	return &Unbound{fs: fs}, nil
}

func (u *Unbound) ValidateConfig(ctx context.Context) error {
	return u.fs.ValidateConfig(ctx)
}

func (u *Unbound) UpdateIps(dnsRecord string, records []internal.ManagedDnsRecord) (bool, error) {
	lines, err := u.fs.ReadConf()
	if err != nil {
		return false, err
	}

	// wantedRecords holds the unbound configuration for the record as key and the line where it is found in the list
	wantedRecords := make(map[string]*int)
	for _, record := range records {
		wantedRecords[recordToUnbound(dnsRecord, record)] = nil
	}

	recordsToRemove := make([]int, 0, len(records))
	for index, line := range lines {
		if strings.HasPrefix(line, fmt.Sprintf(`local-data: "%s `, dnsRecord)) {
			lineContainsWantedRecord := false
			for wantedRecord := range wantedRecords {
				if line == wantedRecord {
					wantedRecords[line] = &index
					lineContainsWantedRecord = true
				}
			}
			// as this line passed the check for the domain but is not a configuration we want, we need to remove it
			if !lineContainsWantedRecord {
				recordsToRemove = append(recordsToRemove, index)
			}
		}
	}

	// iterate over wanted records and collect missing ones
	missingRecords := make([]string, 0, len(wantedRecords))
	for record, line := range wantedRecords {
		if line == nil {
			missingRecords = append(missingRecords, record)
		}
	}

	if len(missingRecords) == 0 && len(recordsToRemove) == 0 {
		return false, nil
	}

	// remove old records
	if len(recordsToRemove) > 0 {
		slog.Warn("Need to remove stale records", "n", len(recordsToRemove), "lines", recordsToRemove)
		lines = removeIndices(lines, recordsToRemove)
	}

	// if we are missing the equal amount of records as the amount of records we want to insert
	if len(missingRecords) == len(records) {
		lines = append(lines, missingRecords...)
	} else {
		idx := 0
		for _, line := range wantedRecords {
			if line != nil {
				idx = *line
			}
		}
		lines = slices.Insert(lines, idx, missingRecords...)
	}

	return true, u.fs.WriteConf(lines)
}

func removeIndices(slice []string, indicesToRemove []int) []string {
	indexMap := make(map[int]struct{}, len(indicesToRemove))
	for _, index := range indicesToRemove {
		indexMap[index] = struct{}{}
	}

	var result []string
	for i, value := range slice {
		if _, exists := indexMap[i]; !exists {
			result = append(result, value)
		}
	}

	return result
}

func recordToUnbound(hostname string, record internal.ManagedDnsRecord) string {
	return fmt.Sprintf(`local-data: "%s %d %s %s"`, hostname, record.Ttl, record.DnsType, record.Ip)
}

func isFileWritable(path string) bool {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}

type FsImpl struct {
	filePath string
}

func NewUnboundConfigWrapper(filePath string, createFile bool) (*FsImpl, error) {
	_, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		if !createFile {
			return nil, fmt.Errorf("unbound config file %q does not exist and createFile=false", filePath)
		} else {
			file, err := os.Create(filePath)
			if err != nil {
				return nil, fmt.Errorf("could not create file %q: %w", filePath, err)
			}
			defer func() {
				_ = file.Close()
			}()
		}
	} else {
		if !isFileWritable(filePath) {
			return nil, fmt.Errorf("unbound config file %q is not writable", filePath)
		}
	}

	return &FsImpl{filePath: filePath}, nil
}

func (u *FsImpl) ReadConf() ([]string, error) {
	oldContent, err := os.ReadFile(u.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read zone file: %v", err)
	}

	return strings.Split(strings.TrimSpace(string(oldContent)), "\n"), nil
}

func (u *FsImpl) WriteConf(conf []string) error {
	//nolint G306
	return os.WriteFile(u.filePath, []byte(strings.Join(conf, "\n")), 0640)
}

func (u *FsImpl) ValidateConfig(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "unbound-checkconf")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unbound-checkconf failed: %w", err)
	}
	return nil
}
