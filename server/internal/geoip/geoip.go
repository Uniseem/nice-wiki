package geoip

import (
	"errors"
	"net/netip"
	"sync"
)

type Range struct {
	Prefix  netip.Prefix
	Country string
}
type Database struct {
	mu     sync.RWMutex
	ranges []Range
}

func New(values []Range) *Database { return &Database{ranges: append([]Range{}, values...)} }
func (d *Database) Lookup(value string) (string, error) {
	address, err := netip.ParseAddr(value)
	if err != nil {
		return "", errors.New("IP 地址无效")
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, entry := range d.ranges {
		if entry.Prefix.Contains(address) {
			return entry.Country, nil
		}
	}
	return "", nil
}
