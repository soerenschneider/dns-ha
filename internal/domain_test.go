package internal

import (
	"net"
	"reflect"
	"slices"
	"testing"
)

func TestComparator(t *testing.T) {
	record0 := ManagedDnsRecord{
		DnsRecord: DnsRecord{
			Priority: 50,
			DnsType:  "A",
			Ip:       net.ParseIP("192.168.1.1"),
			Ttl:      60,
		},
	}

	record1 := ManagedDnsRecord{
		DnsRecord: DnsRecord{
			Priority: 200,
			DnsType:  "A",
			Ip:       net.ParseIP("10.0.0.1"),
			Ttl:      60,
		},
	}

	records := []ManagedDnsRecord{
		record0,
		record1,
	}

	slices.SortFunc(records, PriorityComparator)
	if !reflect.DeepEqual(records[0], record1) {
		t.Errorf("expected %v, got %v", record1, records[0])
	}
}
