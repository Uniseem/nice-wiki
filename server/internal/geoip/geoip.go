package geoip

import (
	"errors"
	"github.com/oschwald/maxminddb-golang/v2"
	"net/netip"
	"os"
	"sync"
)

type Database struct {
	mu     sync.RWMutex
	reader *maxminddb.Reader
}

func Open(path string) (*Database, error) {
	database := &Database{}
	if err := database.Reload(path); errors.Is(err, os.ErrNotExist) {
		return database, nil
	} else if err != nil {
		return nil, err
	}
	return database, nil
}

func (d *Database) Reload(path string) error {
	reader, err := maxminddb.Open(path)
	if err != nil {
		return err
	}
	if err := reader.Verify(); err != nil {
		_ = reader.Close()
		return err
	}
	d.mu.Lock()
	previous := d.reader
	d.reader = reader
	d.mu.Unlock()
	if previous != nil {
		return previous.Close()
	}
	return nil
}

func (d *Database) Lookup(value string) (string, error) {
	address, err := netip.ParseAddr(value)
	if err != nil {
		return "", errors.New("IP 地址无效")
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.reader == nil {
		return "", nil
	}
	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	result := d.reader.Lookup(address.Unmap())
	if !result.Found() {
		return "", nil
	}
	if err := result.Decode(&record); err != nil {
		return "", err
	}
	return record.Country.ISOCode, nil
}

func (d *Database) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.reader == nil {
		return nil
	}
	err := d.reader.Close()
	d.reader = nil
	return err
}
