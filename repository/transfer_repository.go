
package repository

import (
	"errors"

	"wallet-service/models"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransferRepository struct {
	DB *gorm.DB
}

func NewTransferRepository(db *gorm.DB) *TransferRepository {
	return &TransferRepository{DB: db}
}

func (r *TransferRepository) FindIdempotencyRecord(
	key string,
) (*models.IdempotencyRecord, error) {
	var record models.IdempotencyRecord

	err := r.DB.Where("idempotency_key = ?", key).
		First(&record).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (r *TransferRepository) CreateIdempotencyRecord(
	tx *gorm.DB,
	record *models.IdempotencyRecord,
) error {
	return tx.Create(record).Error
}

func (r *TransferRepository) LockWallets(
	tx *gorm.DB,
	walletIDs []string,
) ([]models.Wallet, error) {
	var wallets []models.Wallet

	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", walletIDs).
		Order("id ASC").
		Find(&wallets).Error

	return wallets, err
}

func (r *TransferRepository) CreateTransfer(
	tx *gorm.DB,
	transfer *models.Transfer,
) error {
	return tx.Create(transfer).Error
}

func (r *TransferRepository) UpdateBalance(
	tx *gorm.DB,
	walletID string,
	delta int64,
) error {
	result := tx.Model(&models.Wallet{}).
		Where("id = ?", walletID).
		Update("balance", gorm.Expr("balance + ?", delta))

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("wallet balance update failed")
	}

	return nil
}

func (r *TransferRepository) UpdateTransferStatus(
	tx *gorm.DB,
	transferID uuid.UUID,
	status string,
) error {
	result := tx.Model(&models.Transfer{}).
		Where("id = ?", transferID).
		Update("status", status)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("transfer status update failed")
	}

	return nil
}

func (r *TransferRepository) SaveIdempotencyResponse(
	tx *gorm.DB,
	record *models.IdempotencyRecord,
	transferID uuid.UUID,
	status int,
	body []byte,
) error {
	return tx.Model(record).Updates(map[string]interface{}{
		"transfer_id":     transferID,
		"response_status": status,
		"response_body":   datatypes.JSON(body),
	}).Error
}
