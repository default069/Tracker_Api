package models

import "time"

type TransactionType string

const (
	TransactionIncome  TransactionType = "income"
	TransactionExpense TransactionType = "expense"
)

func (t TransactionType) IsValid() bool {
	return t == TransactionIncome || t == TransactionExpense
}

type Transaction struct {
	ID          uint            `json:"id" gorm:"primaryKey"`
	CategoryID  uint            `json:"category_id" gorm:"not null;index"`
	Category    *Category       `json:"category,omitempty" gorm:"foreignKey:CategoryID;constraint:OnDelete:RESTRICT"`
	Type        TransactionType `json:"type" gorm:"type:varchar(10);not null;index"`
	AmountMinor int64           `json:"amount_minor" gorm:"not null"`
	OccurredAt  time.Time       `json:"occurred_at" gorm:"not null;index"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// TransactionFilter — параметры фильтрации и пагинации.
type TransactionFilter struct {
	From       *time.Time
	To         *time.Time
	CategoryID *uint
	Type       *TransactionType
	Limit      int
	Offset     int
}
