package handle

import (
	"errors"
	"net/http"
	"strings"

	"users/internal/models"

	"github.com/gin-gonic/gin"
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
		respondError(c, 200, err.Error())
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
