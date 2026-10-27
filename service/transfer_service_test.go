package service

import (
	"fmt"
	"sync"
	"testing"

	database "wallet-service/db"
	"wallet-service/models"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func TestConcurrentTransfers(t *testing.T) {
	// Load environment variables.
	if err := godotenv.Load("../.env"); err != nil {
		if err := godotenv.Load(); err != nil {
			t.Fatalf("failed to load .env: %v", err)
		}
	}

	// Connect to the database.
	database, err := database.ConnectDatabase()
	if err != nil {
		t.Fatalf("database connection failed: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("failed to get SQL database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	// Create fresh wallet IDs for this test.
	senderID := "test-sender-" + uuid.NewString()
	receiverID := "test-receiver-" + uuid.NewString()

	// Sender starts with 50000 paise (Rs. 500).
	sender := models.Wallet{
		ID:      senderID,
		Balance: 50000,
	}
	if err := database.Create(&sender).Error; err != nil {
		t.Fatalf("failed to create sender wallet: %v", err)
	}

	// Receiver starts with zero balance.
	receiver := models.Wallet{
		ID:      receiverID,
		Balance: 0,
	}
	if err := database.Create(&receiver).Error; err != nil {
		t.Fatalf("failed to create receiver wallet: %v", err)
	}

	svc := NewTransferService(database)

	var wg sync.WaitGroup
	results := make(chan error, 3)

	// Send three concurrent requests, each transferring 20000 paise.
	for i := 0; i < 3; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			_, transferErr := svc.Transfer(TransferRequest{
				IdempotencyKey: fmt.Sprintf("concurrent-%s-%d", uuid.NewString(), i),
				FromWalletID:   senderID,
				ToWalletID:     receiverID,
				Amount:         20000,
			})

			results <- transferErr
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	failureCount := 0

	for transferErr := range results {
		if transferErr == nil {
			successCount++
			t.Log("Transfer succeeded")
		} else {
			failureCount++
			t.Logf("Transfer failed: %v", transferErr)
		}
	}

	t.Logf("Successful transfers: %d out of 3", successCount)
	t.Logf("Failed transfers: %d out of 3", failureCount)

	// Exactly two transfers should succeed.
	if successCount != 2 {
		t.Fatalf("expected 2 successful transfers, got %d", successCount)
	}

	if failureCount != 1 {
		t.Fatalf("expected 1 failed transfer, got %d", failureCount)
	}

	// Verify final sender balance.
	var finalSender models.Wallet
	if err := database.First(&finalSender, "id = ?", senderID).Error; err != nil {
		t.Fatalf("failed to load sender wallet: %v", err)
	}

	if finalSender.Balance != 10000 {
		t.Fatalf("expected sender balance 10000 paise, got %d", finalSender.Balance)
	}

	// Verify final receiver balance.
	var finalReceiver models.Wallet
	if err := database.First(&finalReceiver, "id = ?", receiverID).Error; err != nil {
		t.Fatalf("failed to load receiver wallet: %v", err)
	}

	if finalReceiver.Balance != 40000 {
		t.Fatalf("expected receiver balance 40000 paise, got %d", finalReceiver.Balance)
	}

	t.Log("Final sender balance: 10000 paise")
	t.Log("Final receiver balance: 40000 paise ")
}
