package models

type Balance struct{
	IncomeMinor  int64
	ExpenseMinor int64
	BalanceMinor int64
}

type MonthlyStat struct{
	Year 		 int	`json:"year"`
	Month 		 int 	`json:"month"`
	IncomeMinor  int64  `json:"income_minor"`
	ExpenseMinor int64	`json:"expense_minor"`
	BalanceMinor int64 	`json:"balance_minor"`
	NetMinor	 int64 	`json:"net_minor"`
}

type CategoryStat struct{
	CategoryID 		uint 	`json:"category_id"`
	CategoryName 	string	`json:"categry_name"`
	TotalMinor 		int64	`json:"total_minor"`
}