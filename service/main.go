package main

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"

	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	repo "github.com/jamesread/SickRock/internal/repo"
	srvpkg "github.com/jamesread/SickRock/internal/server"
	"github.com/jamesread/golure/pkg/dirs"
	_ "modernc.org/sqlite"
)

func getConfigDirectory() string {
	dir, _ := dirs.GetFirstExistingDirectory("config", []string{
		"~/.config/SickRock",
		"/config",
	})

	return dir
}

func loadEnvFile() {
	envFile := filepath.Join(getConfigDirectory(), "SickRock.env")

	// Check if the file exists
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		log.Debugf("Environment file %s not found, skipping", envFile)
		return
	}

	file, err := os.Open(envFile)
	if err != nil {
		log.Warnf("Failed to open environment file %s: %v", envFile, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			log.Warnf("Invalid environment variable format in %s line %d: %s", envFile, lineNum, line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		// Only set if not already set (environment variables take precedence)
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
			log.Debugf("Loaded environment variable: %s", key)
		} else {
			log.Debugf("Environment variable %s already set, skipping", key)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Warnf("Error reading environment file %s: %v", envFile, err)
		return
	}

	log.Infof("Loaded environment variables from %s", envFile)
}

func configureLogging() {
	// Set default log level to debug
	log.SetLevel(log.DebugLevel)

	// Allow override via environment variable
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		switch strings.ToLower(logLevel) {
		case "debug":
			log.SetLevel(log.DebugLevel)
		case "info":
			log.SetLevel(log.InfoLevel)
		case "warn", "warning":
			log.SetLevel(log.WarnLevel)
		case "error":
			log.SetLevel(log.ErrorLevel)
		case "fatal":
			log.SetLevel(log.FatalLevel)
		case "panic":
			log.SetLevel(log.PanicLevel)
		default:
			log.Warnf("Unknown log level '%s', using debug level", logLevel)
		}
	}

	log.Debug("Logging configured")
}

func findFrontendDir() string {
	possibleDirs := []string{
		"../frontend/dist",
		"/www",
		"dist",
	}

	dir, err := dirs.GetFirstExistingDirectory("frontend", possibleDirs)

	if err != nil {
		log.Fatalf("Could not find frontend directory: %v", err)
	}

	return dir
}

// Custom Gin logger middleware that uses logrus
func ginLogrusLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Use logrus to log the request
		log.WithFields(log.Fields{
			"status":     param.StatusCode,
			"method":     param.Method,
			"path":       param.Path,
			"ip":         param.ClientIP,
			"user_agent": param.Request.UserAgent(),
			"latency":    param.Latency,
			"time":       param.TimeStamp.Format(time.RFC3339),
		}).Debug("HTTP Request")

		// Return empty string since we're using logrus directly
		return ""
	})
}

func main() {
	log.Info("SickRock is starting up...")

	loadEnvFile()
	configureLogging()

	gin.SetMode(gin.ReleaseMode)

	// Create router without default middleware
	router := gin.New()

	// Add custom logrus logger middleware
	router.Use(ginLogrusLogger())

	// Add recovery middleware
	router.Use(gin.Recovery())

	// ConnectRPC service mounted under /api
	db, err := repo.OpenFromEnv("file:../tmp/sickrock.db?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	r := repo.NewRepository(db)
	if err := r.EnsureSchema(context.Background()); err != nil {
		log.Fatalf("schema: %v", err)
	}

	srv := srvpkg.NewSickRockServer(r)
	path, handler := sickrockpbconnect.NewSickRockHandler(srv)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	router.Any("/api/*any", gin.WrapH(http.StripPrefix("/api", mux)))

	// SPA fallback for non-API routes
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Status(http.StatusNotFound)
			return
		}
		c.File(filepath.Join(findFrontendDir(), "index.html"))
	})

	// Serve static files from frontend directory (avoiding wildcard conflicts)
	frontendDir := findFrontendDir()
	router.Static("/assets", filepath.Join(frontendDir, "assets"))
	router.Static("/css", filepath.Join(frontendDir, "css"))
	router.Static("/js", filepath.Join(frontendDir, "js"))
	router.Static("/images", filepath.Join(frontendDir, "images"))
	router.StaticFile("/favicon.ico", filepath.Join(frontendDir, "favicon.ico"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Infof("Listening on port %s", port)

	router.Run(":" + port)
}
