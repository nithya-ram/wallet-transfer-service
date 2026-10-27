
package repository

import (
	"wallet-service/models"

	"gorm.io/gorm"
)

type LedgerRepository struct{}

func NewLedgerRepository() *LedgerRepository {
	return &LedgerRepository{}
}

func (r *LedgerRepository) CreateEntries(
	tx *gorm.DB,
	entries []models.LedgerEntry,
) error {
	return tx.Create(&entries).Error
}
