package collector

import (
	"encoding/json"
	"fmt"
	"kolektor/config"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
)

type (
	SystemdCollector struct {
		Nodes  []config.Node
		Ticker *time.Ticker
	}

	Unit struct {
		Name        string    `json:"unit"`
		Timestamp   time.Time `json:"timestamp"`
		Load        string    `json:"load"`
		State       string    `json:"active"`
		Sub         string    `json:"sub"`
		Description string    `json:"description"`
		Label       MetricLabel
		RawLabel    json.RawMessage
	}

	MetricLabel struct {
		Group      string `json:"group"`
		Importance string `json:"importance"`
		Hostname   string `json:"hostname"`
		Remark     string `json:"remark"`
	}
)

func NewSystemdCollector(nodes []config.Node, interval time.Duration) *SystemdCollector {
	return &SystemdCollector{
		Nodes:  nodes,
		Ticker: time.NewTicker(interval),
	}
}

func (sc *SystemdCollector) Run(metrics chan<- interface{}) {
	for range sc.Ticker.C {
		for _, node := range sc.Nodes {
			var statuses []Unit
			var nodeConn string
			if node.Hostname == "" {
				nodeConn = node.IP
			}

			if node.IP == "" {
				nodeConn = node.Hostname
			}

			if nodeConn == "" {
				log.Fatal("Hostname and IP is empty. Need to declare at least one of them.")
			}

			cmd := exec.Command("systemctl", "list-units", "-H", fmt.Sprintf("%s@%s", node.Username, nodeConn), "-o", "json", "--no-pager")

			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("err: %s, %s", err, output)
				continue
			}

			if err := json.Unmarshal(output, &statuses); err != nil {
					log.Error("Error decoding JSON", err)
					return
			}

			var filtered []Unit
			for _, service := range node.Services {
				if len(service.Ports) > 0 {
					if service.Importance == "" {
						service.Importance = "low"
					}
					for _, port := range service.Ports {
						unit := findUnit(statuses, fmt.Sprintf("%s@%d.service", service.Name, port), nodeConn, service)
						filtered = append(filtered, unit)
					}
				} else {
					unit := findUnit(statuses, fmt.Sprintf("%s.service", service.Name), nodeConn, service)
					filtered = append(filtered, unit)
				}
			}
			metrics <- filtered
		}
	}
}

func findUnit(statuses []Unit, name string, hostname string, service config.Service) Unit {
	for _, status := range statuses {
		if status.Name == name {
			status.Timestamp = time.Now()
			status.Label = MetricLabel{
				Group:      service.Group,
				Importance: service.Importance,
				Hostname:   hostname,
			}
			return status
		}
	}
	deadUnit := Unit{
		Name:        name,
		Timestamp:   time.Now(),
		Load:        "",
		State:       "inactive",
		Sub:         "dead",
		Description: "",
		Label: MetricLabel{
			Group:      service.Group,
			Importance: service.Importance,
			Hostname:   hostname,
		},
	}
	return deadUnit
}

// Close implements the Close method for SystemdCollector
func (sc *SystemdCollector) Close() error {
	sc.Ticker.Stop()
	log.Debug("Stopping Systemd Kolektor...")
	return nil
}

// report_id, type_id, requested_by, status, format, digital_signed, generated_at
