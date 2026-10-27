package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"wallet-service/models"
	"wallet-service/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidRequest      = errors.New("invalid transfer request")
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrIdempotencyConflict = errors.New("idempotency key used with different request")
)

type TransferRequest struct {
	IdempotencyKey string `json:"idempotencyKey" binding:"required"`
	FromWalletID   string `json:"fromWalletId" binding:"required"`
	ToWalletID     string `json:"toWalletId" binding:"required"`
	Amount         int64  `json:"amount" binding:"gt=0"`
}

type TransferResponse struct {
	TransferID   string `json:"transferId"`
	FromWalletID string `json:"fromWalletId"`
	ToWalletID   string `json:"toWalletId"`
	Amount       int64  `json:"amount"`
	Status       string `json:"status"`
}

type TransferService struct {
	DB           *gorm.DB
	TransferRepo *repository.TransferRepository
	LedgerRepo   *repository.LedgerRepository
}

func NewTransferService(db *gorm.DB) *TransferService {
	return &TransferService{
		DB:           db,
		TransferRepo: repository.NewTransferRepository(db),
		LedgerRepo:   repository.NewLedgerRepository(),
	}
}

func hashRequest(req TransferRequest) string {
	data, _ := json.Marshal(struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Amount int64  `json:"amount"`
	}{
		From:   req.FromWalletID,
		To:     req.ToWalletID,
		Amount: req.Amount,
	})

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *TransferService) cachedResponse(
	key string,
	requestHash string,
) (*TransferResponse, bool, error) {
	record, err := s.TransferRepo.FindIdempotencyRecord(key)

	fmt.Println(record, "recorddd")

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if record.RequestHash != requestHash {
		return nil, false, ErrIdempotencyConflict
	}

	if len(record.ResponseBody) == 0 {
		return nil, false, errors.New("saved response is unavailable")
	}

	var response TransferResponse
	if err := json.Unmarshal(record.ResponseBody, &response); err != nil {
		return nil, false, err
	}

	return &response, true, nil
}

func (s *TransferService) Transfer(
	req TransferRequest,
) (*TransferResponse, error) {
	if req.IdempotencyKey == "" ||
		req.FromWalletID == "" ||
		req.ToWalletID == "" ||
		req.Amount <= 0 ||
		req.FromWalletID == req.ToWalletID {
		return nil, ErrInvalidRequest
	}

	requestHash := hashRequest(req)

	response, found, err := s.cachedResponse(
		req.IdempotencyKey,
		requestHash,
	)
	if err != nil {
		return nil, err
	}
	if found {
		return response, nil
	}

	transferID := uuid.New()
	var result TransferResponse

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		record := models.IdempotencyRecord{
			IdempotencyKey: req.IdempotencyKey,
			RequestHash:    requestHash,
		}

		if err := s.TransferRepo.CreateIdempotencyRecord(
			tx, &record,
		); err != nil {
			return err
		}

		walletIDs := []string{
			req.FromWalletID,
			req.ToWalletID,
		}
		sort.Strings(walletIDs)

		wallets, err := s.TransferRepo.LockWallets(tx, walletIDs)
		if err != nil {
			return err
		}
		if len(wallets) != 2 {
			return ErrWalletNotFound
		}

		walletMap := make(map[string]*models.Wallet)
		for i := range wallets {
			walletMap[wallets[i].ID] = &wallets[i]
		}

		from := walletMap[req.FromWalletID]
		to := walletMap[req.ToWalletID]

		if from == nil || to == nil {
			return ErrWalletNotFound
		}
		if from.Balance < req.Amount {
			return ErrInsufficientBalance
		}

		transfer := models.Transfer{
			ID:           transferID,
			FromWalletID: req.FromWalletID,
			ToWalletID:   req.ToWalletID,
			Amount:       req.Amount,
			Status:       "PENDING",
		}

		if err := s.TransferRepo.CreateTransfer(
			tx, &transfer,
		); err != nil {
			return err
		}

		if err := s.TransferRepo.UpdateBalance(
			tx, req.FromWalletID, -req.Amount,
		); err != nil {
			return err
		}

		if err := s.TransferRepo.UpdateBalance(
			tx, req.ToWalletID, req.Amount,
		); err != nil {
			return err
		}

		entries := []models.LedgerEntry{
			{
				TransferID: transferID,
				WalletID:   req.FromWalletID,
				EntryType:  "DEBIT",
				Amount:     req.Amount,
			},
			{
				TransferID: transferID,
				WalletID:   req.ToWalletID,
				EntryType:  "CREDIT",
				Amount:     req.Amount,
			},
		}

		if err := s.LedgerRepo.CreateEntries(tx, entries); err != nil {
			return err
		}

		if err := s.TransferRepo.UpdateTransferStatus(
			tx, transferID, "PROCESSED",
		); err != nil {
			return err
		}

		result = TransferResponse{
			TransferID:   transferID.String(),
			FromWalletID: req.FromWalletID,
			ToWalletID:   req.ToWalletID,
			Amount:       req.Amount,
			Status:       "PROCESSED",
		}

		body, err := json.Marshal(result)
		if err != nil {
			return err
		}

		return s.TransferRepo.SaveIdempotencyResponse(
			tx,
			&record,
			transferID,
			200,
			body,
		)
	})

	if err != nil {
		saved, found, lookupErr := s.cachedResponse(
			req.IdempotencyKey,
			requestHash,
		)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if found {
			return saved, nil
		}

		return nil, fmt.Errorf("transfer failed: %w", err)
	}

	return &result, nil
}
