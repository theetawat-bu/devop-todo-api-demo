package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"devops-todo-api/internal/todos"
)

type Options struct {
	AppVersion string
	StartedAt  time.Time
}

func New(pool *pgxpool.Pool, opts Options) *gin.Engine {
	if gin.Mode() == gin.DebugMode && os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// ให้ req.ClientIP() เป็น IP จริงเมื่ออยู่หลัง Nginx / Ingress
	_ = r.SetTrustedProxies(nil)

	r.Use(requestLogger())

	// liveness: แค่บอกว่า process ยังไม่ตาย
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": opts.AppVersion,
			"uptime":  time.Since(opts.StartedAt).Seconds(),
		})
	})

	// readiness: ต่อ DB ได้ไหม (k8s ใช้ตัวนี้ตัดสินใจส่ง traffic เข้ามา)
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not-ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	api := r.Group("/api/todos")
	todos.NewHandler(pool).Register(api)

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	})

	return r
}

// log แบบง่าย ๆ (production จริงควรใช้ zerolog/zap)
func requestLogger() gin.HandlerFunc {
	logger := log.New(os.Stdout, "", 0)
	return func(c *gin.Context) {
		c.Next()
		entry, _ := json.Marshal(gin.H{
			"level":  "info",
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
			"ip":     c.ClientIP(),
			"status": c.Writer.Status(),
		})
		logger.Println(string(entry))
	}
}
