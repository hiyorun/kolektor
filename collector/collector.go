package collector

import (
	"fmt"
	"kolektor/config"
	"time"
)

type Collector interface {
	Run(chan<- interface{})
	Close() error
}

func CollectorFactory(cfg config.Collector, interval time.Duration) (Collector, error) {
	switch cfg.Type {
	case "systemd":
		return NewSystemdCollector(cfg.Nodes, interval), nil
	case "kolektor":
		return NewClientCollector(cfg.Nodes, interval), nil
	default:
		return nil, fmt.Errorf("unsupported collector: %s", cfg.Type)
	}
}
