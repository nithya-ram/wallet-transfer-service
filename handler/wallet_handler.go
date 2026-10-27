package handler

import (
	"net/http"

	"wallet-service/models"
	"wallet-service/repository"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	Repo *repository.WalletRepository
}

func NewWalletHandler(repo *repository.WalletRepository) *WalletHandler {
	return &WalletHandler{Repo: repo}
}

type CreateWalletRequest struct {
	ID      string `json:"id" binding:"required"`
	Balance int64  `json:"balance" binding:"gte=0"`
}

func (h *WalletHandler) CreateWallet(c *gin.Context) {
	var req CreateWalletRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid wallet request",
		})
		return
	}

	wallet := models.Wallet{
		ID:      req.ID,
		Balance: req.Balance,
	}

	if err := h.Repo.CreateWallet(&wallet); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "wallet could not be created; check whether ID already exists",
		})
		return
	}

	c.JSON(http.StatusCreated, wallet)
}
