//go:build linux

package services

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/coreos/go-systemd/v22/dbus"
)

// Collector holds a reused system-bus connection to systemd.
type Collector struct {
	mu   sync.Mutex
	conn *dbus.Conn
}

func New() *Collector { return &Collector{} }

func (c *Collector) connection(ctx context.Context) (*dbus.Conn, error) {
	if c.conn != nil {
		return c.conn, nil
	}
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

func (c *Collector) reset() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Collector) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.connection(context.Background())
	return err == nil
}

func (c *Collector) collect(ctx context.Context) ([]*Service, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	conn, err := c.connection(ctx)
	if err != nil {
		return nil, err
	}
	units, err := conn.ListUnitsByPatternsContext(ctx, []string{}, []string{"*.service"})
	if err != nil {
		c.reset()
		return nil, err
	}
	files, err := conn.ListUnitFilesContext(ctx)
	if err != nil {
		c.reset()
		return nil, err
	}

	rawUnits := make([]rawUnit, 0, len(units))
	for _, u := range units {
		rawUnits = append(rawUnits, rawUnit{Name: u.Name, Description: u.Description, ActiveState: u.ActiveState, SubState: u.SubState})
	}
	rawFiles := make([]rawUnitFile, 0, len(files))
	for _, f := range files {
		rawFiles = append(rawFiles, rawUnitFile{Name: filepath.Base(f.Path), State: f.Type})
	}
	return mergeServices(rawUnits, rawFiles), nil
}

func (c *Collector) CollectInventory(ctx context.Context) ([]*Service, error) {
	return c.collect(ctx)
}

func (c *Collector) CollectHealth(ctx context.Context) ([]*ServiceHealth, error) {
	svcs, err := c.collect(ctx)
	if err != nil {
		return nil, err
	}
	health := make([]*ServiceHealth, 0, len(svcs))
	for _, s := range svcs {
		health = append(health, &ServiceHealth{Name: s.Name, ActiveState: s.ActiveState, SubState: s.SubState})
	}
	return health, nil
}

func (c *Collector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reset()
	return nil
}
