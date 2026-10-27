package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Wallet struct {
	ID        string `gorm:"primaryKey;size:50"`
	Balance   int64  `gorm:"not null;default:0;check:balance >= 0"`
	CreatedAt time.Time
}

type Transfer struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	FromWalletID string    `gorm:"size:50;not null"`
	ToWalletID   string    `gorm:"size:50;not null"`
	Amount       int64     `gorm:"not null;check:amount > 0"`
	Status       string    `gorm:"size:20;not null;default:PENDING"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type LedgerEntry struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	TransferID uuid.UUID `gorm:"type:uuid;not null"`
	WalletID   string    `gorm:"size:50;not null"`
	EntryType  string    `gorm:"size:10;not null"`
	Amount     int64     `gorm:"not null;check:amount > 0"`
	CreatedAt  time.Time
}

type IdempotencyRecord struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement"`
	IdempotencyKey string     `gorm:"size:255;not null;uniqueIndex"`
	RequestHash    string     `gorm:"size:64;not null"`
	TransferID     *uuid.UUID `gorm:"type:uuid"`
	ResponseStatus *int
	ResponseBody   datatypes.JSON
	CreatedAt      time.Time
}

func RunMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&Wallet{},
		&Transfer{},
		&LedgerEntry{},
		&IdempotencyRecord{},
	)
}
