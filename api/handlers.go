package api

import (
	"encoding/json"
	"kolektor/collector"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
)

type healthReport struct {
	Report []timeRange `json:"report"`
}

type timeRange struct {
	Timestamp time.Time   `json:"timestamp"`
	Statuses  []subSystem `json:"statuses"`
}

type subSystem struct {
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

	healthReport, err := h.getSubsystemHealth(timeFrom, timeTo, duration)
	if err != nil {
		log.Error("Can not get latest health report", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res, err := json.Marshal(healthReport)
	if err != nil {
		log.Error("Failed to marshal health latest report", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (h *HTTPServer) getSubsystemHealth(from, to time.Time, interval time.Duration) (healthReport, error) {
	var report healthReport
	for i := from; i.Before(to); i = i.Add(interval) {
		var units []collector.Unit

		query := `
		with latest as (
			select
				ss.name,
				ss.label,
				max(ss.id) as id
			from
				service_status ss
			where
				ss."timestamp" >= ?
			and
				ss."timestamp" <= ?
			group by
				ss.name,
				ss.label
		)
		select
			l.name,
			ss."timestamp",
			ss.load,
			ss.status,
			ss.substatus,
			ss.label
		from
			latest l
		join service_status ss
			using(id);
		`
		rows, err := h.db.Query(query, from.Format(time.RFC3339), i.Add(interval).Format(time.RFC3339))
		if err != nil {
			log.Error("Failed to query historical health", err)
			return HealthReport{}, err
		}

		for rows.Next() {
			var unit collector.Unit
			rows.Scan(&unit.Name, &unit.Timestamp, &unit.Load, &unit.State, &unit.Sub, &unit.RawLabel)
			units = append(units, unit)
		}

		groupedUnits := make(map[string][]collector.Unit)

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

		var healthAtTime timeRange
		for group, units := range groupedUnits {
			if len(units) == 0 {
				healthAtTime.Statuses = append(healthAtTime.Statuses, subSystem{
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
				healthAtTime.Statuses = append(healthAtTime.Statuses, subSystem{
					Name:   group,
					Health: "normal",
				})
			} else if activeUnits == 0 || highImportanceFails {
				healthAtTime.Statuses = append(healthAtTime.Statuses, subSystem{
					Name:   group,
					Health: "down",
				})
			} else {
				healthAtTime.Statuses = append(healthAtTime.Statuses, subSystem{
					Name:   group,
					Health: "degraded",
				})
			}
		}
		healthAtTime.Timestamp = i
		report.Report = append(report.Report, healthAtTime)
	}
	return report, nil
}
