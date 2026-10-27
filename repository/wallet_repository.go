package repository

import (
	"errors"
	"wallet-service/models"

	"gorm.io/gorm"
)

type WalletRepository struct {
	DB *gorm.DB
}

func NewWalletRepository(db *gorm.DB) *WalletRepository {
	return &WalletRepository{
		DB: db,
	}
}

// Create a new wallet
func (repo *WalletRepository) CreateWallet(
	wallet *models.Wallet,
) error {
	return repo.DB.Create(wallet).Error
}

// Get wallet details by ID
func (repo *WalletRepository) GetWalletByID(
	walletID string,
) (*models.Wallet, error) {
	var wallet models.Wallet

	err := repo.DB.First(&wallet, "id = ?", walletID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("wallet not found")
		}
		return nil, err
	}

	return &wallet, nil
}
