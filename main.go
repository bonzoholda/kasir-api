package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL Driver
	"github.com/spf13/viper"
)

type Config struct {
	Port   string
	DBConn string
}

type Product struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
	Price     int    `json:"price"`
	Stock     int    `json:"stock"`
}

var db *sql.DB

func main() {
	// 1. SETUP
	viper.AutomaticEnv()
	if _, err := os.Stat(".env"); err == nil {
		viper.SetConfigFile(".env")
		_ = viper.ReadInConfig()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = viper.GetString("PORT")
	}
	if port == "" {
		port = "8080"
	}

	dbConn := os.Getenv("DB_CONN")
	if dbConn == "" {
		dbConn = viper.GetString("DB_CONN")
	}

	// 2. STABILIZE CONNECTION STRING
	// We force a timeout so the app doesn't hang forever on EOF
	if !strings.Contains(dbConn, "connect_timeout") {
		if strings.Contains(dbConn, "?") {
			dbConn += "&connect_timeout=15"
		} else {
			dbConn += "?connect_timeout=15"
		}
	}

	// 3. CONNECT WITH POOLER LIMITS
	var err error
	db, err = sql.Open("postgres", dbConn)
	if err != nil {
		log.Fatal("Driver error:", err)
	}

	// PROVEN FIX: The pooler hates multiple idle connections from a fresh boot.
	// We set these strictly to allow the handshake to finish.
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	fmt.Printf("Starting connection attempts (Len: %d)...\n", len(dbConn))

	// Robust Retry Logic
	for i := 1; i <= 5; i++ {
		err = db.Ping()
		if err == nil {
			fmt.Println("✅ DATABASE CONNECTED SUCCESSFULLY!")
			break
		}
		fmt.Printf("⚠️ Attempt %d/5: %v. Retrying in 3s...\n", i, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Fatal("❌ Terminal failure: Could not connect to Supabase.")
	}
	defer db.Close()

	// 4. ROUTES
	http.HandleFunc("/api/produk", handleProductCollection)
	http.HandleFunc("/api/produk/", handleProductByID)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := ":" + port
	fmt.Printf("🚀 Server online at %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// --- Handlers (Standard Logic) ---

func handleProductCollection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, created_at, name, price, stock FROM product")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		products := []Product{}
		for rows.Next() {
			var p Product
			rows.Scan(&p.ID, &p.CreatedAt, &p.Name, &p.Price, &p.Stock)
			products = append(products, p)
		}
		json.NewEncoder(w).Encode(products)
	} else if r.Method == "POST" {
		var p Product
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		query := `INSERT INTO product (name, price, stock) VALUES ($1, $2, $3) RETURNING id, created_at`
		err := db.QueryRow(query, p.Name, p.Price, p.Stock).Scan(&p.ID, &p.CreatedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

func handleProductByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/produk/")
	id, _ := strconv.Atoi(idStr)
	if r.Method == "GET" {
		var p Product
		err := db.QueryRow("SELECT id, created_at, name, price, stock FROM product WHERE id = $1", id).Scan(&p.ID, &p.CreatedAt, &p.Name, &p.Price, &p.Stock)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(p)
	}
}
