package handler

import (
	"errors"
	"net/http"

	"wallet-service/service"

	"github.com/gin-gonic/gin"
)

type TransferHandler struct {
	Service *service.TransferService
}

func NewTransferHandler(s *service.TransferService) *TransferHandler {
	return &TransferHandler{Service: s}
}

func (h *TransferHandler) CreateTransfer(c *gin.Context) {
	var req service.TransferRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	response, err := h.Service.Transfer(req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrWalletNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInsufficientBalance):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrIdempotencyConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "transfer could not be completed",
			})
		}
		return
	}

	c.JSON(http.StatusOK, response)
}
