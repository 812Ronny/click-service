package db

import (
	"TestProject1/config"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	Conn *sql.DB
}

func NewDB(config *config.Config) (*DB, error) {
	connStr := config.GetDBConnectionString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(50)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %v", err)
	}

	log.Println("Connected to database successfully")

	return &DB{Conn: db}, nil
}

func (db *DB) Close() {
	if err := db.Conn.Close(); err != nil {
		log.Printf("error closing database connection: %v", err)
	} else {
		log.Println("Database connection closed successfully")
	}
}
