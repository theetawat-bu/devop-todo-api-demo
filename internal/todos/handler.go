package todos

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Register(r gin.IRouter) {
	r.GET("", h.list)
	r.GET("/:id", h.get)
	r.POST("", h.create)
	r.PATCH("/:id", h.update)
	r.DELETE("/:id", h.delete)
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed"})
		return 0, false
	}
	return id, true
}

func (h *Handler) list(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, title, done, created_at, updated_at FROM todos ORDER BY id DESC`)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	defer rows.Close()

	result := []Todo{}
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.UpdatedAt); err != nil {
			_ = c.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		result = append(result, t)
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var t Todo
	err := h.pool.QueryRow(c.Request.Context(),
		`SELECT id, title, done, created_at, updated_at FROM todos WHERE id = $1`, id,
	).Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, t)
}

func (h *Handler) create(c *gin.Context) {
	var in CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	done := false
	if in.Done != nil {
		done = *in.Done
	}

	var t Todo
	err := h.pool.QueryRow(c.Request.Context(),
		`INSERT INTO todos (title, done) VALUES ($1, $2)
		 RETURNING id, title, done, created_at, updated_at`, in.Title, done,
	).Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, t)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var in UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	var t Todo
	err := h.pool.QueryRow(c.Request.Context(),
		`UPDATE todos SET
			title = COALESCE($1, title),
			done = COALESCE($2, done),
			updated_at = now()
		 WHERE id = $3
		 RETURNING id, title, done, created_at, updated_at`, in.Title, in.Done, id,
	).Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, t)
}

func (h *Handler) delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	tag, err := h.pool.Exec(c.Request.Context(), `DELETE FROM todos WHERE id = $1`, id)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
