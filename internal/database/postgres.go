package database

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
)

func ConnectPostgres() (*sql.DB, error) {
	connStr := "user=myuser password=secret dbname=messages_1_db sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	log.Println("Connected to PostgreSQL")
	return db, nil
}

func SaveMessage(db *sql.DB, content string) (int, error) {
	var id int
	err := db.QueryRow("INSERT INTO messages (content, status) VALUES ($1, 'pending') RETURNING id", content).Scan(&id)
	return id, err
}

func UpdateMessageStatus(db *sql.DB, id int, status string) error {
	_, err := db.Exec("UPDATE messages SET status = $1 WHERE id = $2", status, id)
	return err
}

func GetProcessedMessageCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM messages WHERE status = 'processed'").Scan(&count)
	return count, err
}
