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

	"kasir-api/handlers"
	"kasir-api/repositories"
	"kasir-api/services"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/viper"
)

type Product struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
	Price     int    `json:"price"`
	Stock     int    `json:"stock"`
}

var db *sql.DB

// Middleware for universal logging - this is your best friend for debugging Railway
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("📢 [%s] %s - From: %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func main() {
	// --- ENV SETUP ---
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
	if dbConn == "" {
		log.Fatal("❌ DB_CONN is not set")
	}

	if !strings.Contains(dbConn, "connect_timeout") {
		if strings.Contains(dbConn, "?") {
			dbConn += "&connect_timeout=15"
		} else {
			dbConn += "?connect_timeout=15"
		}
	}

	// --- DB CONNECT ---
	var err error
	db, err = sql.Open("pgx", dbConn)
	if err != nil {
		log.Fatal("Driver error:", err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	fmt.Printf("Starting connection attempts...\n")
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

	// --- INITIALIZE HANDLERS ---
	txRepo := repositories.NewTransactionRepository(db)
	txService := services.NewTransactionService(db, txRepo)
	txHandler := handlers.NewTransactionHandler(txService)

	// --- ROUTES SETUP ---
	mux := http.NewServeMux()

	// Products
	mux.HandleFunc("/api/produk", handleProductCollection)
	mux.HandleFunc("/api/produk/", handleProductByID)

	// ✅ CHECKOUT ROUTE: Mapped to both slash and no-slash for maximum safety
	mux.HandleFunc("/api/checkout", txHandler.HandleCheckout)
	mux.HandleFunc("/api/checkout/", txHandler.HandleCheckout)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 🔍 CATCH-ALL: Log any path that doesn't match the above
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("❓ 404 ALERT: Route not found for [%s] %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	addr := ":" + port
	fmt.Printf("🚀 Server online at %s\n", addr)

	// Apply the global logger middleware to the mux
	log.Fatal(http.ListenAndServe(addr, requestLogger(mux)))
}

// --- PRODUCT HANDLERS ---

func handleProductCollection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		name := r.URL.Query().Get("name")
		query := "SELECT id, created_at, name, price, stock FROM product"
		var args []interface{}

		if name != "" {
			query += " WHERE name ILIKE $1"
			args = append(args, "%"+name+"%")
		}
		query += " ORDER BY id DESC"

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var products []Product
		for rows.Next() {
			var p Product
			if err := rows.Scan(&p.ID, &p.CreatedAt, &p.Name, &p.Price, &p.Stock); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			products = append(products, p)
		}
		json.NewEncoder(w).Encode(products)

	case "POST":
		var p Product
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		err := db.QueryRow(
			`INSERT INTO product (name, price, stock)
             VALUES ($1, $2, $3)
             RETURNING id, created_at`,
			p.Name, p.Price, p.Stock,
		).Scan(&p.ID, &p.CreatedAt)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleProductByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/produk/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var p Product
	err = db.QueryRow(
		`SELECT id, created_at, name, price, stock
         FROM product
         WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.CreatedAt, &p.Name, &p.Price, &p.Stock)

	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(p)
}
