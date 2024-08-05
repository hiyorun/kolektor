package api

import (
	"encoding/json"
	"kolektor/collector"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
)

type HealthReport struct {
	Report []StatusTimeFrame `json:"report"`
}

type StatusTimeFrame struct {
	Timestamp time.Time         `json:"timestamp"`
	Statuses  []SubSystemStatus `json:"statuses"`
}

type SubSystemStatus struct {
	Name   string `json:"name"`
	Health string `json:"health"`
}

func (h *HTTPServer) SysHealth(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	interval := r.URL.Query().Get("interval")

	timeFrom, err := time.Parse(time.RFC3339, from)
	if err != nil {
		timeFrom = time.Now().Add(-time.Hour * 24)
	}
	timeTo, err := time.Parse(time.RFC3339, to)
	if err != nil {
		timeTo = time.Now()
	}
	duration, err := time.ParseDuration(interval)
	if err != nil {
		duration = time.Hour
	}

	HealthReport, err := h.getSubsystemHealth(timeFrom, timeTo, duration)
	if err != nil {
		log.Error("Can not get historical health report", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res, err := json.Marshal(HealthReport)
	if err != nil {
		log.Error("Failed to marshal health historical report", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (h *HTTPServer) LatestSysHealth(w http.ResponseWriter, r *http.Request) {
	HealthReport, err := h.getSubsystemHealth(time.Now(), time.Now(), time.Hour)
	if err != nil {
		log.Error("Can not get latest health report", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res, err := json.Marshal(HealthReport)
	if err != nil {
		log.Error("Failed to marshal health latest report", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (h *HTTPServer) getSubsystemHealth(from, to time.Time, interval time.Duration) (HealthReport, error) {
	var report HealthReport

	query := `
	WITH latest AS (
		SELECT
			ss.name,
			ss.label,
			MAX(ss.id) AS id
		FROM
			service_status ss
		WHERE
			ss.timestamp >= datetime(?,'unixepoch','localtime')
			AND ss.timestamp <= datetime(?,'unixepoch','localtime')
		GROUP BY
			ss.name,
			ss.label
	)
	SELECT
		l.name,
		ss.timestamp,
		ss.load,
		ss.status,
		ss.substatus,
		ss.label
	FROM
		latest l
	JOIN service_status ss
		ON l.id = ss.id;
	`

	groupedUnits := make(map[string][]collector.Unit)

	for i := from; i.Before(to); i = i.Add(interval) {
		var units []collector.Unit

		rows, err := h.db.Query(query, i.Add(-interval).Unix(), i.Unix())
		if err != nil {
			log.Error("Failed to query historical health", err)
			return HealthReport{}, err
		}

		for rows.Next() {
			var unit collector.Unit
			rows.Scan(&unit.Name, &unit.Timestamp, &unit.Load, &unit.State, &unit.Sub, &unit.RawLabel)
			units = append(units, unit)
		}

		for _, collecter := range h.cfg.Collectors {
			for _, node := range collecter.Nodes {
				for _, service := range node.Services {
					groupedUnits[service.Group] = []collector.Unit{}
				}
			}
		}

		for _, unit := range units {
			if err := json.Unmarshal(unit.RawLabel, &unit.Label); err != nil {
				log.Error("Error parsing label", err)
				return HealthReport{}, err
			}
			group := unit.Label.Group
			groupedUnits[group] = append(groupedUnits[group], unit)
		}

		var status StatusTimeFrame
		for group, units := range groupedUnits {
			if len(units) == 0 {
				status.Statuses = append(status.Statuses, SubSystemStatus{
					Name:   group,
					Health: "none",
				})
				continue
			}
			activeUnits := 0
			highImportanceFails := false
			for _, unit := range units {
				if unit.State != "active" {
					if unit.Label.Importance == "high" {
						highImportanceFails = true
					}
				}
				if unit.State == "active" {
					activeUnits++
				}
			}
			if activeUnits == len(units) {
				status.Statuses = append(status.Statuses, SubSystemStatus{
					Name:   group,
					Health: "normal",
				})
			} else if activeUnits == 0 || highImportanceFails {
				status.Statuses = append(status.Statuses, SubSystemStatus{
					Name:   group,
					Health: "down",
				})
			} else {
				status.Statuses = append(status.Statuses, SubSystemStatus{
					Name:   group,
					Health: "degraded",
				})
			}
		}
		status.Timestamp = i
		report.Report = append(report.Report, status)
	}
	return report, nil
}
