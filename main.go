package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Produk represents a product in the cashier system
type Produk struct {
	ID    int    `json:"id"`
	Nama  string `json:"nama"`
	Harga int    `json:"harga"`
	Stok  int    `json:"stok"`
}

// In-memory storage
var produk = []Produk{
	{ID: 1, Nama: "Indomie Godog", Harga: 3500, Stok: 10},
	{ID: 2, Nama: "Vit 1000ml", Harga: 3000, Stok: 40},
	{ID: 3, Nama: "kecap", Harga: 12000, Stok: 20},
}

// Handler for GET (one), PUT, and DELETE
func handleProdukWithID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/produk/")
	if idStr == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid Produk ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		getProdukByID(w, id)
	case "PUT":
		updateProduk(w, r, id)
	case "DELETE":
		deleteProduk(w, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getProdukByID(w http.ResponseWriter, id int) {
	for _, p := range produk {
		if p.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(p)
			return
		}
	}
	http.Error(w, "Produk belum ada", http.StatusNotFound)
}

func updateProduk(w http.ResponseWriter, r *http.Request, id int) {
	var updatedData Produk
	err := json.NewDecoder(r.Body).Decode(&updatedData)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	for i := range produk {
		if produk[i].ID == id {
			updatedData.ID = id
			produk[i] = updatedData
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(updatedData)
			return
		}
	}
	http.Error(w, "Produk belum ada", http.StatusNotFound)
}

func deleteProduk(w http.ResponseWriter, id int) {
	for i, p := range produk {
		if p.ID == id {
			produk = append(produk[:i], produk[i+1:]...)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "sukses delete"})
			return
		}
	}
	http.Error(w, "Produk belum ada", http.StatusNotFound)
}

func main() {
	// 1. Health Check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "OK",
			"message": "API Running",
		})
	})

	// 2. Collection Route (GET all, POST new)
	http.HandleFunc("/api/produk", func(w http.ResponseWriter, r *http.Request) {
		// Strict check for exact path to avoid colliding with /api/produk/
		if r.URL.Path != "/api/produk" {
			http.NotFound(w, r)
			return
		}

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(produk)
		} else if r.Method == "POST" {
			var produkBaru Produk
			if err := json.NewDecoder(r.Body).Decode(&produkBaru); err != nil {
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}
			produkBaru.ID = len(produk) + 1
			produk = append(produk, produkBaru)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(produkBaru)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 3. Item Route (GET by ID, PUT, DELETE)
	http.HandleFunc("/api/produk/", handleProdukWithID)

	fmt.Println("Server running di localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("gagal running server: %v\n", err)
	}
}