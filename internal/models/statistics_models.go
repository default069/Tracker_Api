package models

// Balance — доход, расход и баланс за период.
type Balance struct {
	IncomeMinor  int64 `json:"income_minor"`
	ExpenseMinor int64 `json:"expense_minor"`
	BalanceMinor int64 `json:"balance_minor"`
}

// MonthlyStat — статистика за месяц.
type MonthlyStat struct {
	Year         int   `json:"year"`
	Month        int   `json:"month"`
	IncomeMinor  int64 `json:"income_minor"`
	ExpenseMinor int64 `json:"expense_minor"`
	BalanceMinor int64 `json:"balance_minor"`
	NetMinor     int64 `json:"net_minor"`
}

// CategoryStat — сумма по категории (не таблица БД, только результат запроса).
type CategoryStat struct {
	CategoryID   uint   `json:"category_id"`
	CategoryName string `json:"category_name"`
	TotalMinor   int64  `json:"total_minor"`
}
