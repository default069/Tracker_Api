package handle

import (
	"net/http"
	"strconv"
	"time"

	"users/internal/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) registerStatisticsRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/statistics")
	g.GET("/balance", h.GetBalance)
	g.GET("/monthly", h.GetMonthlyStats)
	g.GET("/categories", h.GetCategoryStats)
}

// GET /statistics/balance?from=&to=
// Общий доход, расход и баланс (доход − расход) за период. Без параметров — за всё время.
func (h *Handler) GetBalance(c *gin.Context) {
	f, msg := parseTransactionFilter(c)
	if msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	// для баланса важен только период
	f.CategoryID, f.Type = nil,

	var b models.Balance
	err := applyTransactionFilter(h.DB.WithContext(c.Request.Context()).Model(&models.Transaction{}), f).
		Select(`COALESCE(SUM(CASE WHEN type = 'income'  THEN amount_minor ELSE 0 END), 0) AS income_minor,
		        COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_minor ELSE 0 END), 0) AS expense_minor`).
		Scan(&b).Error
	if err != nil {
		respondInternal(c, err)
		return
	}
	b.BalanceMinor = b.IncomeMinor - b.ExpenseMinor
	c.JSON(http.StatusOK, b)
}

// GET /statistics/monthly?year=2026
// 12 месяцев года (месяцы без операций — нули, считаем в UTC).
//   - net_minor     — доход − расход за месяц;
//   - balance_minor — накопленный баланс на конец месяца (с учётом всех операций до начала года).
func (h *Handler) GetMonthlyStats(c *gin.Context) {
	year := time.Now().UTC().Year()
	if v := c.Query("year"); v != "" {
		y, err := strconv.Atoi(v)
		if err != nil || y < 1970 || y > 9999 {
			respondError(c, http.StatusBadRequest, "invalid 'year'")
			return
		}
		year = y
	}
	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)

	db := h.DB.WithContext(c.Request.Context())

	// баланс на начало года
	var opening struct{ Net int64 }
	err := db.Model(&models.Transaction{}).
		Where("occurred_at < ?", start).
		Select(`COALESCE(SUM(CASE WHEN type = 'income' THEN amount_minor ELSE -amount_minor END), 0) AS net`).
		Scan(&opening).Error
	if err != nil {
		respondInternal(c, err)
		return
	}

	var rows []struct {
		Month        int
		IncomeMinor  int64
		ExpenseMinor int64
	}
	err = db.Model(&models.Transaction{}).
		Where("occurred_at >= ? AND occurred_at < ?", start, end).
		Select(`CAST(EXTRACT(MONTH FROM occurred_at AT TIME ZONE 'UTC') AS int) AS month,
		        COALESCE(SUM(CASE WHEN type = 'income'  THEN amount_minor ELSE 0 END), 0) AS income_minor,
		        COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_minor ELSE 0 END), 0) AS expense_minor`).
		Group("month").
		Scan(&rows).Error
	if err != nil {
		respondInternal(c, err)
		return
	}

	byMonth := make(map[int][2]int64, len(rows))
	for _, r := range rows {
		byMonth[r.Month] = [2]int64{r.IncomeMinor, r.ExpenseMinor}
	}

	stats := make([]models.MonthlyStat, 0, 12)
	running := opening.Net
	for m := 1; m <= 12; m++ {
		v := byMonth[m]
		net := v[0] - v[1]
		running += net
		stats = append(stats, models.MonthlyStat{
			Year:         year,
			Month:        m,
			IncomeMinor:  v[0],
			ExpenseMinor: v[1],
			NetMinor:     net,
			BalanceMinor: running,
		})
	}
	c.JSON(http.StatusOK, stats)
}

// GET /statistics/categories?type=expense&from=&to=
// Суммы по категориям за период, по убыванию. type по умолчанию 'expense'.
func (h *Handler) GetCategoryStats(c *gin.Context) {
	f, msg := parseTransactionFilter(c)
	if msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	typ := models.TransactionExpense
	if f.Type != nil {
		typ = *f.Type
	}
	f.Type = nil
	f.CategoryID = nil

	stats := []models.CategoryStat{}
	err := applyTransactionFilter(
		h.DB.WithContext(c.Request.Context()).Table("transactions"), f).
		Where("transactions.type = ?", typ).
		Joins("JOIN categories ON categories.id = transactions.category_id").
		Select(`categories.id AS category_id,
		        categories.name AS category_name,
		        SUM(transactions.amount_minor) AS total_minor`).
		Group("categories.id, categories.name").
		Order("total_minor DESC, categories.name ASC").
		Scan(&stats).Error
	if err != nil {
		respondInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}