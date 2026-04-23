package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
        "os"
	"fmt"
        "github.com/coreos/go-systemd/v22/daemon"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/lib/pq"
)

var db *sql.DB

type Note struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func initDB() {
    dbUser := getEnv("DB_USER", "devops")
    dbPassword := getEnv("DB_PASSWORD", "secret")
    dbName := getEnv("DB_NAME", "notes")
    dbHost := getEnv("DB_HOST", "localhost")
    dbPort := getEnv("DB_PORT", "5432")
    dbSSLMode := getEnv("DB_SSLMODE", "disable")

    connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=%s",
        dbUser, dbPassword, dbName, dbHost, dbPort, dbSSLMode)

    var err error
    db, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal("Ошибка открытия БД:", err)
    }
}
func enableCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	}
}

func getNotes(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, content, created_at FROM notes ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		err := rows.Scan(&n.ID, &n.Content, &n.CreatedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		notes = append(notes, n)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notes)
}

func addNote(w http.ResponseWriter, r *http.Request) {
	var note Note
	err := json.NewDecoder(r.Body).Decode(&note)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Получена заметка: %s\n", note.Content)

	err = db.QueryRow(
		"INSERT INTO notes (content) VALUES ($1) RETURNING id, created_at",
		note.Content,
	).Scan(&note.ID, &note.CreatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Вставлена заметка с ID=%d\n", note.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

func getEnv(key, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
    err := db.Ping()
    if err != nil {
        log.Printf("Health check failed: %v", err)
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("Database unavailable"))
        return
    }
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func main() {
	initDB()
	defer db.Close()
         
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/notes", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			getNotes(w, r)
		case "POST":
			addNote(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	http.HandleFunc("/health", healthCheck)
	 daemon.SdNotify(false, daemon.SdNotifyReady)

    // Горутина для поддержки watchdog
    go func() {
        interval, err := daemon.SdWatchdogEnabled(false)
        if err != nil || interval == 0 {
            log.Println("Watchdog не включен или не поддерживается")
            return
        }
        ticker := time.NewTicker(interval / 2)
        defer ticker.Stop()
        for range ticker.C {
            daemon.SdNotify(false, daemon.SdNotifyWatchdog)
        }
    }()

	port := ":8080"
	log.Printf("Сервер запущен на http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))}
