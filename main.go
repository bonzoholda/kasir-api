package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Config struct for Viper
type Config struct {
	Port   string `mapstructure:"PORT"`
	DBConn string `mapstructure:"DB_CONN"`
}

// Produk struct
type Produk struct {
	ID    int    `json:"id"`
	Nama  string `json:"nama"`
	Harga int    `json:"harga"`
	Stok  int    `json:"stok"`
}

// Keeping the in-memory storage for now so the code runs
var produk = []Produk{
	{ID: 1, Nama: "Indomie Godog", Harga: 3500, Stok: 10},
	{ID: 2, Nama: "Vit 1000ml", Harga: 3000, Stok: 40},
}

func main() {
	// 1. SETUP VIPER
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if _, err := os.Stat(".env"); err == nil {
		viper.SetConfigFile(".env")
		_ = viper.ReadInConfig()
	}

	// Default values if .env is missing
	viper.SetDefault("PORT", "8080")

	config := Config{
		Port:   viper.GetString("PORT"),
		DBConn: viper.GetString("DB_CONN"),
	}

	// 2. DATABASE (Note: This requires your 'database' package to exist)
	/* db, err := database.InitDB(config.DBConn)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()
	*/

	// 3. ROUTES
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
	})

	// Collection Route
	http.HandleFunc("/api/produk", handleProdukCollection)

	// Item Route
	http.HandleFunc("/api/produk/", handleProdukWithID)

	// 4. START SERVER
	addr := ":" + config.Port // Railway needs the colon format
	fmt.Println("Server running di port", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("gagal running server: %v", err)
	}
}

// --- Handlers ---

func handleProdukCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(produk)
	} else if r.Method == "POST" {
		var p Produk
		json.NewDecoder(r.Body).Decode(&p)
		p.ID = len(produk) + 1
		produk = append(produk, p)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

func handleProdukWithID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/produk/")
	id, _ := strconv.Atoi(idStr)

	switch r.Method {
	case "GET":
		for _, p := range produk {
			if p.ID == id {
				json.NewEncoder(w).Encode(p)
				return
			}
		}
		http.NotFound(w, r)
	case "DELETE":
		// ... delete logic ...
		w.Write([]byte("Deleted"))
	}
}
