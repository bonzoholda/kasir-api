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
	"time" // Needed for retry sleep

	_ "github.com/lib/pq" // PostgreSQL Driver
	"github.com/spf13/viper"
)

// Config struct
type Config struct {
	Port   string
	DBConn string
}

// Product matches your database schema exactly
type Product struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
	Price     int    `json:"price"`
	Stock     int    `json:"stock"`
}

var db *sql.DB // Global database connection

func main() {
	// 1. SETUP VIPER & ENV
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

	config := Config{
		Port:   port,
		DBConn: dbConn,
	}

	// 2. CONNECT TO DATABASE (With Retry Logic)
	if config.DBConn == "" {
		log.Fatal("CRITICAL: DB_CONN is empty. Check Railway settings.")
	}

	var err error
	db, err = sql.Open("postgres", config.DBConn)
	if err != nil {
		log.Fatal("Error opening database driver:", err)
	}

	// PROVEN SETTINGS FOR SUPABASE POOLER:
	// Limits help prevent the "EOF" error by not overwhelming the pooler
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute * 5)

	// Retry Ping logic: Gives Railway network time to stabilize
	fmt.Printf("Starting DB connection attempts (String Length: %d)\n", len(config.DBConn))
	for i := 1; i <= 5; i++ {
		err = db.Ping()
		if err == nil {
			fmt.Println("✅ Successfully connected to Database!")
			break
		}
		fmt.Printf("⚠️ Attempt %d/5: Database unreachable, retrying in 2s... (%v)\n", i, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatal("❌ Failed to connect to database after 5 attempts. Check your DB_CONN string.")
	}
	defer db.Close()

	// 3. ROUTES
	http.HandleFunc("/api/produk", handleProductCollection)
	http.HandleFunc("/api/produk/", handleProductByID)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := ":" + config.Port
	fmt.Println("🚀 Server running on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// --- Handlers ---

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
		query := `SELECT id, created_at, name, price, stock FROM product WHERE id = $1`
		err := db.QueryRow(query, id).Scan(&p.ID, &p.CreatedAt, &p.Name, &p.Price, &p.Stock)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(p)
	}
}
