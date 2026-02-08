package repositories

import (
	"database/sql"
	"fmt"
	"kasir-api/models"
)

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// ✅ Accepts *sql.Tx passed from the Service
func (repo *TransactionRepository) CreateTransaction(tx *sql.Tx, items []models.CheckoutItem) (*models.Transaction, error) {
	totalAmount := 0
	details := make([]models.TransactionDetail, 0)

	for _, item := range items {
		var productPrice, currentStock int
		var productName string

		// 1. Fetch current data and Lock the row for update to prevent race conditions
		err := tx.QueryRow("SELECT name, price, stock FROM product WHERE id = $1 FOR UPDATE", item.ProductID).
			Scan(&productName, &productPrice, &currentStock)

		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("product id %d not found", item.ProductID)
		}
		if err != nil {
			return nil, err
		}

		// 2. Check if stock is sufficient
		if currentStock < item.Quantity {
			return nil, fmt.Errorf("insufficient stock for %s (available: %d, requested: %d)", productName, currentStock, item.Quantity)
		}

		subtotal := productPrice * item.Quantity
		totalAmount += subtotal

		// 3. Deduct stock
		_, err = tx.Exec("UPDATE product SET stock = stock - $1 WHERE id = $2", item.Quantity, item.ProductID)
		if err != nil {
			return nil, err
		}

		details = append(details, models.TransactionDetail{
			ProductID:   item.ProductID,
			ProductName: productName,
			Quantity:    item.Quantity,
			Subtotal:    subtotal,
		})
	}

	// 4. Record the main Transaction
	var transactionID int
	err := tx.QueryRow("INSERT INTO transactions (total_amount) VALUES ($1) RETURNING id", totalAmount).Scan(&transactionID)
	if err != nil {
		return nil, err
	}

	// 5. Record Transaction Details
	for i := range details {
		details[i].TransactionID = transactionID
		_, err = tx.Exec("INSERT INTO transaction_details (transaction_id, product_id, quantity, subtotal) VALUES ($1, $2, $3, $4)",
			transactionID, details[i].ProductID, details[i].Quantity, details[i].Subtotal)
		if err != nil {
			return nil, err
		}
	}

	return &models.Transaction{
		ID:          transactionID,
		TotalAmount: totalAmount,
		Details:     details,
	}, nil
}
