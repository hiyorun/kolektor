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
	ClientCollector struct {
		Nodes  []config.Node
		Ticker *time.Ticker
	}
)

func NewClientCollector(nodes []config.Node, interval time.Duration) *ClientCollector {
	return &ClientCollector{
		Nodes:  nodes,
		Ticker: time.NewTicker(interval),
	}
}

func (sc *ClientCollector) Run(metrics chan<- interface{}) {
	for range sc.Ticker.C {
		for _, node := range sc.Nodes {
			go func(node config.Node, metrics chan<- interface{}) {
				nodeConn := node.Hostname
				if node.IP != "" {
					nodeConn = node.IP
				} else if nodeConn == "" {
					log.Fatal("Hostname and IP is empty. Need to declare at least one of them.")
				}

				var services []string
				for _, service := range node.Services {
					serviceStr := fmt.Sprintf("%s.service", service.Name)
					if len(service.Suffix) > 0 {
						for _, suffix := range service.Suffix {
							serviceStr = fmt.Sprintf("%s@%s.service", service.Name, suffix)
							services = append(services, serviceStr)
						}
						continue
					}
					services = append(services, serviceStr)
				}

				srv, err := json.Marshal(services)
				if err != nil {
					log.Error("Error marshaling JSON", err)
				}

				log.Debug("Get status", "node", nodeConn, "services", string(srv))
				cmd := exec.Command("ssh", fmt.Sprintf("%s@%s", node.Username, nodeConn), "kolektor-client", "--services", fmt.Sprintf("'%s'", string(srv)))

				output, err := cmd.Output()
				if err != nil {
					log.Debug("Can not get status", "node", nodeConn, err, "output", string(output))
					output = json.RawMessage("[]")
				}

				var statuses []Unit
				if err := json.Unmarshal(output, &statuses); err != nil {
					log.Error("Error decoding JSON", err)
					return
				}

				for i := range statuses {
					statuses[i].Timestamp = time.Now()
					for _, service := range node.Services {
						if len(service.Suffix) > 0 {
							for _, suffix := range service.Suffix {
								if statuses[i].Name == fmt.Sprintf("%s@%s.service", service.Name, suffix) {
									statuses[i].Label = MetricLabel{
										Group:      service.Group,
										Importance: service.Importance,
										Hostname:   nodeConn,
									}
								}
							}
						} else {
							if statuses[i].Name == fmt.Sprintf("%s.service", service.Name) {
								statuses[i].Label = MetricLabel{
									Group:      service.Group,
									Importance: service.Importance,
									Hostname:   nodeConn,
								}
							}
						}
					}
				}
				log.Debug("Got status", "node", nodeConn, "status", fmt.Sprint(statuses))

				metrics <- statuses
			}(node, metrics)
		}
	}
}

// Close implements the Close method for ClientCollector
func (sc *ClientCollector) Close() error {
	sc.Ticker.Stop()
	log.Debug("Stopping Systemd Kolektor...")
	return nil
}
