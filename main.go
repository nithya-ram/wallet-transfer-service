package main

import (
	"log"
	database "wallet-service/db"
	"wallet-service/handler"
	"wallet-service/models"
	"wallet-service/repository"
	"wallet-service/service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	db, err := database.ConnectDatabase()
	if err != nil {
		panic(err)
	}

	if err := models.RunMigrations(db); err != nil {
		panic(err)
	}

	walletRepo := repository.NewWalletRepository(db)
	walletHandler := handler.NewWalletHandler(walletRepo)

	transferService := service.NewTransferService(db)
	transferHandler := handler.NewTransferHandler(transferService)

	router := gin.Default()

	router.POST("/wallets", walletHandler.CreateWallet)
	router.POST("/transfers", transferHandler.CreateTransfer)

	log.Println("Wallet Transfer Service running on :8080")

	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}

}
