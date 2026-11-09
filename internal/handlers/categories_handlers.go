package handle

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"users/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Handler — общий контейнер зависимостей для всех HTTP-обработчиков.
type Handler struct {
	DB *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{DB: db}
}

// RegisterRoutes подключает все маршруты приложения к группе (например r.Group("/api")).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	h.registerCategoryRoutes(rg)
	h.registerTransactionRoutes(rg)
	h.registerStatisticsRoutes(rg)
}

func (h *Handler) registerCategoryRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/categories")
	g.POST("", h.CreateCategory)
	g.GET("", h.ListCategories)
	g.GET("/:id", h.GetCategory)
	g.PUT("/:id", h.UpdateCategory)
	g.DELETE("/:id", h.DeleteCategory)
}

// ErrorResponse — тело ошибки (используется в swagger-описании).
type ErrorResponse struct {
	Error string `json:"error" example:"invalid id"`
}

// ---------- общие хелперы (используются всеми handlers) ----------

func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

func respondInternal(c *gin.Context, err error) {
	// детали ошибки в ответ не отдаём, только в лог gin
	_ = c.Error(err)
	respondError(c, http.StatusInternalServerError, "internal server error")
}

func parseIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondError(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return uint(id), true
}

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func isUniqueViolation(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) || pgErrCode(err) == "23505"
}

func isForeignKeyViolation(err error) bool {
	return errors.Is(err, gorm.ErrForeignKeyViolated) || pgErrCode(err) == "23503"
}

// parseTimeParam принимает RFC3339 или YYYY-MM-DD (UTC).
// dateOnly == true, если передана только дата.
func parseTimeParam(s string) (t time.Time, dateOnly bool, err error) {
	if t, err = time.Parse(time.RFC3339, s); err == nil {
		return t, false, nil
	}
	if t, err = time.Parse("2006-01-02", s); err == nil {
		return t, true, nil
	}
	return time.Time{}, false, errors.New("expected RFC3339 or YYYY-MM-DD")
}

// ---------- categories ----------

type categoryInput struct {
	Name string `json:"name" binding:"required"`
}

func (in *categoryInput) normalize() (string, string) {
	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		return "", "name must not be empty"
	case len([]rune(name)) > 100:
		return "", "name must be at most 100 characters"
	}
	return name, ""
}

// POST /categories
//
// @Summary      Создать категорию
// @Description  Имя уникально, до 100 символов.
// @Tags         categories
// @Accept       json
// @Produce      json
// @Param        input body categoryInput true "Данные категории"
// @Success      201 {object} models.Category
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      409 {object} ErrorResponse "Конфликт"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /categories [post]
func (h *Handler) CreateCategory(c *gin.Context) {
	var in categoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	name, msg := in.normalize()
	if msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}

	cat := models.Category{Name: name}
	if err := h.DB.WithContext(c.Request.Context()).Create(&cat).Error; err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "category with this name already exists")
			return
		}
		respondInternal(c, err)
		return
	}
	c.JSON(http.StatusCreated, cat)
}

// GET /categories
//
// @Summary      Список категорий
// @Description  Отсортирован по имени.
// @Tags         categories
// @Produce      json
// @Success      200 {array} models.Category
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /categories [get]
func (h *Handler) ListCategories(c *gin.Context) {
	var cats []models.Category
	if err := h.DB.WithContext(c.Request.Context()).Order("name ASC").Find(&cats).Error; err != nil {
		respondInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, cats)
}

// GET /categories/:id
//
// @Summary      Получить категорию по ID
// @Tags         categories
// @Produce      json
// @Param        id path int true "ID"
// @Success      200 {object} models.Category
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      404 {object} ErrorResponse "Не найдено"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /categories/{id} [get]
func (h *Handler) GetCategory(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var cat models.Category
	if err := h.DB.WithContext(c.Request.Context()).First(&cat, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "category not found")
			return
		}
		respondInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, cat)
}

// PUT /categories/:id
//
// @Summary      Обновить категорию
// @Tags         categories
// @Accept       json
// @Produce      json
// @Param        id path int true "ID"
// @Param        input body categoryInput true "Новое имя"
// @Success      200 {object} models.Category
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      404 {object} ErrorResponse "Не найдено"
// @Failure      409 {object} ErrorResponse "Конфликт"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /categories/{id} [put]
func (h *Handler) UpdateCategory(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var in categoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	name, msg := in.normalize()
	if msg != "" {
		respondError(c, http.StatusBadRequest, msg)
		return
	}

	db := h.DB.WithContext(c.Request.Context())
	var cat models.Category
	if err := db.First(&cat, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "category not found")
			return
		}
		respondInternal(c, err)
		return
	}

	if err := db.Model(&cat).Update("name", name).Error; err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "category with this name already exists")
			return
		}
		respondInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, cat)
}

// DELETE /categories/:id
// Нельзя удалить категорию, у которой есть транзакции (FK ON DELETE RESTRICT).
//
// @Summary      Удалить категорию
// @Description  Нельзя удалить категорию, у которой есть транзакции.
// @Tags         categories
// @Produce      json
// @Param        id path int true "ID"
// @Success      204 "Удалено"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      404 {object} ErrorResponse "Не найдено"
// @Failure      409 {object} ErrorResponse "Конфликт"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /categories/{id} [delete]
func (h *Handler) DeleteCategory(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	res := h.DB.WithContext(c.Request.Context()).Delete(&models.Category{}, id)
	if res.Error != nil {
		if isForeignKeyViolation(res.Error) {
			respondError(c, http.StatusConflict, "category has transactions and cannot be deleted")
			return
		}
		respondInternal(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "category not found")
		return
	}
	c.Status(http.StatusNoContent)
}
