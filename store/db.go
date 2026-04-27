package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// Review represents a single AI code review stored in the database
type Review struct {
	ID         int       `json:"id"`
	Repo       string    `json:"repo"`
	PRNumber   int       `json:"pr_number"`
	PRTitle    string    `json:"pr_title"`
	Filename   string    `json:"filename"`
	Language   string    `json:"language"`
	ReviewText string    `json:"review_text"`
	CreatedAt  time.Time `json:"created_at"`
}

// DB wraps the database connection
type DB struct {
	conn *sql.DB
}

// New connects to PostgreSQL using DATABASE_URL from environment
func New() (*DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("pinging db: %w", err)
	}

	log.Println("Connected to PostgreSQL")
	return &DB{conn: conn}, nil
}

// Migrate creates the reviews table if it doesn't exist
func (db *DB) Migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS reviews (
			id          SERIAL PRIMARY KEY,
			repo        TEXT NOT NULL,
			pr_number   INTEGER NOT NULL,
			pr_title    TEXT NOT NULL,
			filename    TEXT NOT NULL,
			language    TEXT NOT NULL,
			review_text TEXT NOT NULL,
			created_at  TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}

// SaveReview stores a single file review in the database
func (db *DB) SaveReview(r Review) error {
	_, err := db.conn.Exec(`
		INSERT INTO reviews (repo, pr_number, pr_title, filename, language, review_text)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, r.Repo, r.PRNumber, r.PRTitle, r.Filename, r.Language, r.ReviewText)
	return err
}

// GetReviews returns all reviews ordered by most recent first
func (db *DB) GetReviews() ([]Review, error) {
	rows, err := db.conn.Query(`
		SELECT id, repo, pr_number, pr_title, filename, language, review_text, created_at
		FROM reviews
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(
			&r.ID, &r.Repo, &r.PRNumber, &r.PRTitle,
			&r.Filename, &r.Language, &r.ReviewText, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, nil
}
