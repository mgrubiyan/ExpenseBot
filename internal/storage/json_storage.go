package storage

import (
	"ExpenseBot/internal/models"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type JSONStorage struct {
	filePath string
	mu       sync.Mutex
}

func NewJSONStorage(filePath string) *JSONStorage {
	dir := filepath.Dir(filePath)
	_ = os.MkdirAll(dir, 0755)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		_ = os.WriteFile(filePath, []byte("[]"), 0644)
	}
	return &JSONStorage{filePath: filePath}
}

func (s *JSONStorage) loadExpenses() ([]models.Expense, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, err
	}

	var expenses []models.Expense

	if err := json.Unmarshal(data, &expenses); err != nil {
		return nil, err
	}

	return expenses, nil
}

func (s *JSONStorage) saveExpenses(expenses []models.Expense) error {
	data, err := json.MarshalIndent(expenses, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return err
	}
	return nil
}

func (s *JSONStorage) AddExpense(expense models.Expense) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	expenses, err := s.loadExpenses()
	if err != nil {
		return err
	}

	var maxID int64
	for _, e := range expenses {
		if e.ID > maxID {
			maxID = e.ID
		}
	}

	expense.ID = maxID + 1

	expenses = append(expenses, expense)

	return s.saveExpenses(expenses)
}
