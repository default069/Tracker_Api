package models

import "time"

// Category — категория транзакций.
type Category struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:100;not null;uniqueIndex"`
	CreatedAt time.Time `json:"created_at"`
}
