package db

import (
	"database/sql"
	"kolektor/config"

	_ "github.com/mattn/go-sqlite3"
)

var sqlite *sql.DB

func DB() *sql.DB {
	return sqlite
}

func Open(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", cfg.Database.Path)
	if err != nil {
		return nil, err
	}

	sqlite = db
	return db, nil
}
