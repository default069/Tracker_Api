package handle

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"users/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

func (h *Handler) registerTransactionRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/transactions")
	g.POST("", h.CreateTransaction)
	g.GET("", h.ListTransactions)
	g.GET("/:id", h.GetTransaction)
	g.PUT("/:id", h.UpdateTransaction)
	g.DELETE("/:id", h.DeleteTransaction)
}

type transactionInput struct {
	CategoryID  uint                   `json:"category_id" binding:"required"`
	Type        models.TransactionType `json:"type" binding:"required"`
	AmountMinor int64                  `json:"amount_minor" binding:"required"`
	OccurredAt  *time.Time             `json:"occurred_at"` // по умолчанию — текущее время
}

func (in *transactionInput) validate() string {
	if !in.Type.IsValid() {
		return "type must be 'income' or 'expense'"
	}
	if in.AmountMinor <= 0 {
		return "amount_minor must be greater than 0"
	}
	return ""
}

// categoryExists проверяет существование категории.
func (h *Handler) categoryExists(c *gin.Context, id uint) (bool, error) {
	var n int64
	err := h.DB.WithContext(c.Request.Context()).
		Model(&models.Category{}).Where("id = ?", id).Count(&n).Error
	return n > 0, err
}

// POST /transactions
func (h *Handler) CreateTransaction(c *gin.Context) {
	var in transactionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}
	exists, err := h.categoryExists(c, in.CategoryID)
	if err != nil {
		respondInternal(c, err)
		return
	}
	if !exists {
		respondError(c, http.StatusUnprocessableEntity, "category does not exist")
		return
	}

	occurred := time.Now().UTC()
	if in.OccurredAt != nil {
		occurred = in.OccurredAt.UTC()
	}
	tx := models.Transaction{
		CategoryID:  in.CategoryID,
		Type:        in.Type,
		AmountMinor: in.AmountMinor,
		OccurredAt:  occurred,
	}
	db := h.DB.WithContext(c.Request.Context())
	if err := db.Create(&tx).Error; err != nil {
		if isForeignKeyViolation(err) {
			respondError(c, http.StatusUnprocessableEntity, "category does not exist")
			return
		}
		respondInternal(c, err)
		return
	}
	// подгружаем категорию для ответа
	if err := db.Preload("Category").First(&tx, tx.ID).Error; err != nil {
		respondInternal(c, err)
		return
	}
	c.JSON(http.StatusCreated, tx)
}

// parseTransactionFilter разбирает query-параметры:
// from, to (RFC3339 или YYYY-MM-DD; to для даты — включительно по весь день),
// category_id, type, limit, offset.
func parseTransactionFilter(c *gin.Context) (models.TransactionFilter, string) {
	f := models.TransactionFilter{Limit: defaultPageLimit}

	if v := c.Query("from"); v != "" {
		t, _, err := parseTimeParam(v)
		if err != nil {
			return f, "invalid 'from': " + err.Error()
		}
		f.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, dateOnly, err := parseTimeParam(v)
		if err != nil {
			return f, "invalid 'to': " + err.Error()
		}
		if dateOnly {
			t = t.AddDate(0, 0, 1) // включаем весь указанный день
		}
		f.To = &t
	}
	if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
		return f, "'from' must be before 'to'"
	}
	if v := c.Query("category_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil || id == 0 {
			return f, "invalid 'category_id'"
		}
		cid := uint(id)
		f.CategoryID = &cid
	}
	if v := c.Query("type"); v != "" {
		t := models.TransactionType(v)
		if !t.IsValid() {
			return f, "'type' must be 'income' or 'expense'"
		}
		f.Type = &t
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return f, "invalid 'limit'"
		}
		if n > maxPageLimit {
			n = maxPageLimit
		}
		f.Limit = n
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return f, "invalid 'offset'"
		}
		f.Offset = n
	}
	return f, ""
}

func applyTransactionFilter(db *gorm.DB, f models.TransactionFilter) *gorm.DB {
	if f.From != nil {
		db = db.Where("occurred_at >= ?", *f.From)
	}
	if f.To != nil {
		db = db.Where("occurred_at < ?", *f.To)
	}
	if f.CategoryID != nil {
		db = db.Where("category_id = ?", *f.CategoryID)
	}
	if f.Type != nil {
		db = db.Where("type = ?", *f.Type)
	}
	return db
}

// GET /transactions
func (h *Handler) ListTransactions(c *gin.Context) {
	f, msg := parseTransactionFilter(c)
	if msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}

	db := h.DB.WithContext(c.Request.Context())

	var total int64
	if err := applyTransactionFilter(db.Model(&models.Transaction{}), f).Count(&total).Error; err != nil {
		respondInternal(c, err)
		return
	}

	items := []models.Transaction{}
	err := applyTransactionFilter(db, f).
		Preload("Category").
		Order("occurred_at DESC, id DESC").
		Limit(f.Limit).Offset(f.Offset).
		Find(&items).Error
	if err != nil {
		respondInternal(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   items,
		"total":  total,
		"limit":  f.Limit,
		"offset": f.Offset,
	})
}

// GET /transactions/:id
func (h *Handler) GetTransaction(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var tx models.Transaction
	if err := h.DB.WithContext(c.Request.Context()).Preload("Category").First(&tx, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "transaction not found")
			return
		}
		respondInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, tx)
}

// PUT /transactions/:id — полная замена полей (occurred_at, если не передан, не меняется).
func (h *Handler) UpdateTransaction(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var in transactionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}

	db := h.DB.WithContext(c.Request.Context())
	var tx models.Transaction
	if err := db.First(&tx, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "transaction not found")
			return
		}
		respondInternal(c, err)
		return
	}

	exists, err := h.categoryExists(c, in.CategoryID)
	if err != nil {
		respondInternal(c, err)
		return
	}
	if !exists {
		respondError(c, http.StatusUnprocessableEntity, "category does not exist")
		return
	}

	updates := map[string]any{
		"category_id":  in.CategoryID,
		"type":         in.Type,
		"amount_minor": in.AmountMinor,
	}
	if in.OccurredAt != nil {
		updates["occurred_at"] = in.OccurredAt.UTC()
	}
	if err := db.Model(&tx).Updates(updates).Error; err != nil {
		if isForeignKeyViolation(err) {
			respondError(c, http.StatusUnprocessableEntity, "category does not exist")
			return
		}
		respondInternal(c, err)
		return
	}
	if err := db.Preload("Category").First(&tx, tx.ID).Error; err != nil {
		respondInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, tx)
}

// DELETE /transactions/:id
func (h *Handler) DeleteTransaction(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	res := h.DB.WithContext(c.Request.Context()).Delete(&models.Transaction{}, id)
	if res.Error != nil {
		respondInternal(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "transaction not found")
		return
	}
	c.Status(http.StatusNoContent)
}