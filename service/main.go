package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	srvpkg "github.com/jamesread/SickRock/internal/server"
	"github.com/jamesread/SickRock/internal/auth"
	repo "github.com/jamesread/SickRock/internal/repo"
)

func ginLogrusLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s [%s] %s %s %d %s %s %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.ErrorMessage,
		)
	})
}

func loadEnvFile() {
	envFile := ".env"
	if _, err := os.Stat(envFile); err == nil {
		file, err := os.Open(envFile)
		if err != nil {
			log.Warnf("Could not open .env file: %v", err)
			return
		}
		defer file.Close()

		// Simple env file parser
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}
}

func configureLogging() {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	logLevel, err := log.ParseLevel(level)
	if err != nil {
		logLevel = log.InfoLevel
	}
	log.SetLevel(logLevel)

	// Set log format
	format := os.Getenv("LOG_FORMAT")
	if format == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
	}
}

func findFrontendDir() string {
	// Try to find the frontend directory
	possiblePaths := []string{
		"../frontend",
		"frontend",
		"./frontend",
		"../../frontend",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(filepath.Join(path, "index.html")); err == nil {
			return path
		}
	}

	// Fallback to current directory
	return "."
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
	
	// Create auth service
	authService := auth.NewAuthService()
	
	// Create ConnectRPC handler with auth interceptor
	interceptors := connect.WithInterceptors(auth.ConnectAuthMiddleware(authService))
	path, handler := sickrockpbconnect.NewSickRockHandler(srv, interceptors)
	
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
