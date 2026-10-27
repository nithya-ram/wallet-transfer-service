package service

import (
	"testing"

	database "wallet-service/db"
	"wallet-service/models"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func setupTransferTest(t *testing.T, senderBalance int64) (*gorm.DB, *TransferService, string, string) {
	t.Helper()

	if err := godotenv.Load("../.env"); err != nil {
		t.Fatalf("failed to load .env: %v", err)
	}

	db, err := database.ConnectDatabase()
	if err != nil {
		t.Fatalf("database connection failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get SQL database: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	senderID := "test-sender-" + uuid.NewString()
	receiverID := "test-receiver-" + uuid.NewString()

	if err := db.Create(&models.Wallet{
		ID:      senderID,
		Balance: senderBalance,
	}).Error; err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}

	if err := db.Create(&models.Wallet{
		ID:      receiverID,
		Balance: 0,
	}).Error; err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}

	return db, NewTransferService(db), senderID, receiverID
}

func checkBalance(t *testing.T, db *gorm.DB, walletID string, want int64) {
	t.Helper()

	var wallet models.Wallet
	if err := db.First(&wallet, "id = ?", walletID).Error; err != nil {
		t.Fatalf("failed to load wallet: %v", err)
	}

	if wallet.Balance != want {
		t.Fatalf("wallet %s: expected balance %d, got %d",
			walletID, want, wallet.Balance)
	}
}

func ledgerCount(t *testing.T, db *gorm.DB, walletID string) int64 {
	t.Helper()

	var count int64
	err := db.Model(&models.LedgerEntry{}).
		Where("wallet_id = ?", walletID).
		Count(&count).Error

	if err != nil {
		t.Fatalf("failed to count ledger entries: %v", err)
	}

	return count
}

func TestTransferExecution(t *testing.T) {
	db, svc, senderID, receiverID := setupTransferTest(t, 50000)

	_, err := svc.Transfer(TransferRequest{
		IdempotencyKey: "test-" + uuid.NewString(),
		FromWalletID:   senderID,
		ToWalletID:     receiverID,
		Amount:         20000,
	})
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	checkBalance(t, db, senderID, 30000)
	checkBalance(t, db, receiverID, 20000)
}

func TestIdempotency(t *testing.T) {
	db, svc, senderID, receiverID := setupTransferTest(t, 50000)

	req := TransferRequest{
		IdempotencyKey: "test-" + uuid.NewString(),
		FromWalletID:   senderID,
		ToWalletID:     receiverID,
		Amount:         20000,
	}

	if _, err := svc.Transfer(req); err != nil {
		t.Fatalf("first transfer failed: %v", err)
	}

	if _, err := svc.Transfer(req); err != nil {
		t.Fatalf("duplicate request failed: %v", err)
	}

	checkBalance(t, db, senderID, 30000)
	checkBalance(t, db, receiverID, 20000)

	if got := ledgerCount(t, db, senderID); got != 1 {
		t.Fatalf("expected 1 sender ledger entry, got %d", got)
	}
	if got := ledgerCount(t, db, receiverID); got != 1 {
		t.Fatalf("expected 1 receiver ledger entry, got %d", got)
	}

	// Same key with different request details must fail.
	req.Amount = 10000
	if _, err := svc.Transfer(req); err == nil {
		t.Fatal("expected conflict for reused key with different amount")
	}

	checkBalance(t, db, senderID, 30000)
	checkBalance(t, db, receiverID, 20000)
}

func TestLedgerCorrectness(t *testing.T) {
	db, svc, senderID, receiverID := setupTransferTest(t, 50000)

	_, err := svc.Transfer(TransferRequest{
		IdempotencyKey: "test-" + uuid.NewString(),
		FromWalletID:   senderID,
		ToWalletID:     receiverID,
		Amount:         20000,
	})
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	if got := ledgerCount(t, db, senderID); got != 1 {
		t.Fatalf("expected one sender ledger entry, got %d", got)
	}
	if got := ledgerCount(t, db, receiverID); got != 1 {
		t.Fatalf("expected one receiver ledger entry, got %d", got)
	}

	// Verify entry types and amounts.
	var debitCount, creditCount int64

	if err := db.Model(&models.LedgerEntry{}).
		Where("wallet_id = ? AND entry_type = ? AND amount = ?",
			senderID, "DEBIT", 20000).
		Count(&debitCount).Error; err != nil {
		t.Fatalf("failed to query debit entry: %v", err)
	}

	if err := db.Model(&models.LedgerEntry{}).
		Where("wallet_id = ? AND entry_type = ? AND amount = ?",
			receiverID, "CREDIT", 20000).
		Count(&creditCount).Error; err != nil {
		t.Fatalf("failed to query credit entry: %v", err)
	}

	if debitCount != 1 || creditCount != 1 {
		t.Fatalf("expected one DEBIT and one CREDIT of 20000; got debit=%d credit=%d",
			debitCount, creditCount)
	}
}

func TestInsufficientBalance(t *testing.T) {
	db, svc, senderID, receiverID := setupTransferTest(t, 10000)

	_, err := svc.Transfer(TransferRequest{
		IdempotencyKey: "test-" + uuid.NewString(),
		FromWalletID:   senderID,
		ToWalletID:     receiverID,
		Amount:         20000,
	})
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}

	// Failed transfer must not change balances or create ledger entries.
	checkBalance(t, db, senderID, 10000)
	checkBalance(t, db, receiverID, 0)

	if got := ledgerCount(t, db, senderID); got != 0 {
		t.Fatalf("expected no sender ledger entries, got %d", got)
	}
	if got := ledgerCount(t, db, receiverID); got != 0 {
		t.Fatalf("expected no receiver ledger entries, got %d", got)
	}
}
