package handle

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

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

func isUniqueViolation(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) || pgErrCode(err) == "23505"
}

func isForeignKeyViolation(err error) bool {
	return errors.Is(err, gorm.ErrForeignKeyViolated) || pgErrCode(err) == "23503"
}
