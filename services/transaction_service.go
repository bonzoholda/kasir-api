package services

import (
	"context"
	"database/sql"
	"fmt"
	"kasir-api/models"
	"kasir-api/repositories"
)

type TransactionService struct {
	db   *sql.DB
	repo *repositories.TransactionRepository
}

func NewTransactionService(db *sql.DB, repo *repositories.TransactionRepository) *TransactionService {
	return &TransactionService{
		db:   db,
		repo: repo,
	}
}

func (s *TransactionService) Checkout(items []models.CheckoutItem, useLock bool) (*models.Transaction, error) {
	// 1. Start a Database Transaction
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not begin transaction: %v", err)
	}

	// 2. Defer a rollback in case of failure.
	// If Commit() is called, this does nothing.
	defer tx.Rollback()

	// 3. Call the repository to handle the DB logic using the transaction 'tx'
	// Note: We pass 'tx' instead of the global 'db'
	transaction, err := s.repo.CreateTransaction(tx, items)
	if err != nil {
		return nil, err
	}

	// 4. If everything is successful, Commit the changes!
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("could not commit transaction: %v", err)
	}

	return transaction, nil
}
