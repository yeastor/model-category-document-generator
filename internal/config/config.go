package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port          string
	BaseDir       string
	DataDir       string
	FieldDir      string
	TemplateDir   string
	ModerationDir string
	PublicDir     string
	OutputDir     string
	PythonBin     string
	LogLevel      string
}

func NewConfig() (*Config, error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dataDir := env("DATA_DIR", filepath.Join(baseDir, "data"))
	return &Config{
		Port:          env("APP_PORT", env("PORT", "4177")),
		BaseDir:       baseDir,
		DataDir:       dataDir,
		FieldDir:      env("FIELD_DIR", filepath.Join(dataDir, "fields")),
		TemplateDir:   env("TEMPLATE_DIR", filepath.Join(baseDir, "document-templates")),
		ModerationDir: env("MODERATION_DIR", filepath.Join(dataDir, "moderation-lists")),
		PublicDir:     env("PUBLIC_DIR", filepath.Join(baseDir, "public")),
		OutputDir:     env("OUTPUT_DIR", filepath.Join(baseDir, "generated")),
		PythonBin:     env("PYTHON_BIN", defaultPythonBin()),
		LogLevel:      env("LOG_LEVEL", "info"),
	}, nil
}

func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (c *Config) Address() string {
	if _, err := strconv.Atoi(c.Port); err == nil {
		return ":" + c.Port
	}
	return c.Port
}

func defaultPythonBin() string {
	candidates := []string{
		filepath.Join(os.Getenv("USERPROFILE"), ".cache", "codex-runtimes", "codex-primary-runtime", "dependencies", "python", "python.exe"),
		"python3",
		"python",
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate, string(filepath.Separator)) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			continue
		}
		return candidate
	}
	return "python"
}
func env(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
