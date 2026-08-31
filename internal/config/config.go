package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
	AppVersion  string
}

// Load อ่านค่าจาก env แล้วโยน error ทันทีถ้าไม่มี DATABASE_URL
// pattern นี้เรียกว่า fail fast — แอปตายตั้งแต่ boot ดีกว่าไปตายตอนมี request จริง
// (บน k8s pod จะเข้า CrashLoopBackOff ให้เห็นเลยว่าตั้งค่าผิด)
func Load() (Config, error) {
	cfg := Config{
		Port:        envOr("PORT", "3000"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AppVersion:  envOr("APP_VERSION", "dev"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is not set")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
