package store

import (
	"database/sql"
	"encoding/json"
	"kolektor/collector"
	"kolektor/config"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type (
	Store interface {
		Run()
		Close()
	}

	SkuyliteStore struct {
		dataChan <-chan interface{}
		db       *sql.DB
		cfg      *config.Store
	}
)

func NewSkuyliteStore(dataChan <-chan interface{}, cfg *config.Store, db *sql.DB) SkuyliteStore {
	return SkuyliteStore{
		dataChan: dataChan,
		db:       db,
		cfg:      cfg,
	}
}

func (ss *SkuyliteStore) Run() {
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS service_status (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME,
			type VARCHAR(100),
			name VARCHAR(100),
			load VARCHAR(100),
			status VARCHAR(100),
			substatus VARCHAR(100),
			label JSON(200)
		);
	`

	_, err := ss.db.Exec(createTableQuery)
	if err != nil {
		log.Fatal("Error while creating table:", err.Error())
		return
	}

	for data := range ss.dataChan {
		ss.processDataFromCollector(data)
	}
}

func (ss *SkuyliteStore) processDataFromCollector(data interface{}) {
	switch v := data.(type) {
	case []collector.Unit:
		go ss.saveSystemdData(v)
	default:
		log.Printf("Unsupported data type: %T", v)
	}
}

func (ss *SkuyliteStore) saveSystemdData(data []collector.Unit) {
	if ss.cfg.Retention == time.Duration(0) {
		ss.cfg.Retention = time.Duration(24) * time.Hour
	}

	delete := `
	delete from
		service_status
	where
		"timestamp" <= datetime(?, 'auto', 'localtime')
	`

	_, err := ss.db.Exec(delete, time.Now().Add(-ss.cfg.Retention))
	if err != nil {
		log.Printf("Error deleting data: %v", err)
	}

	for _, unit := range data {
		label, err := json.Marshal(unit.Label)
		if err != nil {
			log.Printf("Error marshalling label: %v", err)
		}

		// Check for last state if config saveChange is true
		if ss.cfg.OnChange {
			query := `
				with named as (
					select
						*
					from
						service_status ss
					where
						name = ?
						and label like ?
					order by
						"timestamp" desc
					limit 1
				)
				select
					n.name,
					n.load,
					n.status,
					n.substatus,
					n.label
				from
					named n
				where 
					n."type" = 'systemd'
					and n.load = 'loaded'
					and n.status = 'active'
					and n.substatus = 'running';
			`
			dataToCheck := []interface{}{unit.Name, label, "systemd", unit.Load, unit.State, unit.Sub}

			// Execute the query
			row := ss.db.QueryRow(query, dataToCheck...)
			var scanner collector.Unit

			// Check if there is a matching row
			err = row.Scan(&scanner.Name, &scanner.Load, &scanner.State, &scanner.Sub, &scanner.RawLabel)
			if err == nil {
				continue
			} else if err != sql.ErrNoRows {
				log.Fatal(err)
			}
		}

		_, err = ss.db.Exec("INSERT INTO service_status (id, timestamp, type, name, load, status, substatus, label) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", nil, unit.Timestamp, "systemd", unit.Name, unit.Load, unit.State, unit.Sub, label)
		if err != nil {
			log.Printf("Error inserting data into SQLite: %v", err)
		}
	}
}

func (ss *SkuyliteStore) Close() {
}
