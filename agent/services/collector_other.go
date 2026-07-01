//go:build !linux

package services

import "context"

// Collector is a no-op outside Linux/systemd.
type Collector struct{}

func New() *Collector { return &Collector{} }

func (c *Collector) IsAvailable() bool { return false }

func (c *Collector) CollectInventory(ctx context.Context) ([]*Service, error) { return nil, nil }

func (c *Collector) CollectHealth(ctx context.Context) ([]*ServiceHealth, error) { return nil, nil }

func (c *Collector) Close() error { return nil }
