package db

import (
	"log"

	"users/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitPostgres() *gorm.DB {
	dsn := "postgres://postgres:1111@localhost:5432/tracker_db?sslmode=disable"

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}
	log.Println("connected")

	// Category раньше Transaction — из-за внешнего ключа.
	if err := database.AutoMigrate(&models.CategoryStat{}, &models.Transaction{}); err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}

	return database
}
