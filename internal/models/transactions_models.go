package models

import (
	"time"
)

type TransactionType string

const (
	TransactionIncome 	TransactionType = "income"
	TransactionExpense TransactionType = "expense"
)

func (t TransactionType) IsValid() bool {
	return 	t == TransactionIncome || t == TransactionExpense
}

type Transaction struct {
	ID  				uint `json:"id" gorm:"primaryKey"`
	CategoryID 			uint `json:"category_id" gorm:"not null; index"`
	Category 			*categories `json:"category,omitempty" gorm:"foreignKey:CategoryID;constraint:OnDelete:RESTRICT"`
	Type 				TransactionType `json:"type" gorm:"type:varchar(10);not null;index;check:type IN ('income','expense')"`
	AmountMinor  		int64 `json:"amount_minor" gorm:"not null;check:amount_minor > 0"`
	OccurredAt 			time.Time `json:"occurred_at" gorm:"not null; index"`
	CreatedAt 			time.Time `json:"created_at"` 
	UpdatedAT 			time.Time `json:"updated_at"`
}

type TransactionFilter struct {
	From 		*time.Time
	To 			*time.Time
	CategoryID 	*uint
	Type   		int
	Offset 		int
}