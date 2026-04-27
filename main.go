package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/GowthamU7/ai-code-reviewer/handler"
	"github.com/GowthamU7/ai-code-reviewer/store"
	"github.com/joho/godotenv"
)

var db *store.DB

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	// Connect to database (optional — app works without it)
	var err error
	db, err = store.New()
	if err != nil {
		log.Printf("Database not available: %v — reviews won't be stored", err)
	} else {
		if err := db.Migrate(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		handler.DB = db
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", handler.Webhook)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/reviews", handleReviews)

	fmt.Printf("Server starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func handleReviews(w http.ResponseWriter, r *http.Request) {
	// Allow React dashboard to call this API
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if db == nil {
		json.NewEncoder(w).Encode([]store.Review{})
		return
	}

	reviews, err := db.GetReviews()
	if err != nil {
		http.Error(w, "failed to fetch reviews", http.StatusInternalServerError)
		return
	}

	if reviews == nil {
		reviews = []store.Review{}
	}

	json.NewEncoder(w).Encode(reviews)
}
