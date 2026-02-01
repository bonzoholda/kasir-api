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

	_ "github.com/lib/pq" // PostgreSQL Driver
	"github.com/spf13/viper"
)

// Config struct
type Config struct {
	Port   string `mapstructure:"PORT"`
	DBConn string `mapstructure:"DB_CONN"`
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
	// 1. SETUP VIPER
	viper.AutomaticEnv()
	if _, err := os.Stat(".env"); err == nil {
		viper.SetConfigFile(".env")
		_ = viper.ReadInConfig()
	}
	viper.SetDefault("PORT", "8080")

	config := Config{
		Port:   viper.GetString("PORT"),
		DBConn: viper.GetString("DB_CONN"), // Should be DATABASE_URL on Railway
	}

	// 2. CONNECT TO DATABASE
	var err error
	db, err = sql.Open("postgres", config.DBConn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Database unreachable:", err)
	}

	// 3. ROUTES
	http.HandleFunc("/api/produk", handleProductCollection)
	http.HandleFunc("/api/produk/", handleProductByID)

	addr := ":" + config.Port
	fmt.Println("Server running on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// --- Handlers using SQL ---

func handleProductCollection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, created_at, name, price, stock FROM product")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var products []Product
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

		// Insert into "product" table
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
