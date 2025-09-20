package main

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/gin-gonic/gin"
	"github.com/jamesread/golure/pkg/dirs"
	log "github.com/sirupsen/logrus"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	"github.com/jamesread/SickRock/internal/auth"
	repo "github.com/jamesread/SickRock/internal/repo"
	srvpkg "github.com/jamesread/SickRock/internal/server"
)

func ginLogrusLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		log.WithFields(log.Fields{
			"path":   param.Path,
			"method": param.Method,
			"status": param.StatusCode,
		}).Debugf("Gin Log")
		return ""
	})
}

func loadEnvFile() {
	envFile, err := dirs.GetFirstExistingFileFromDirs("env", []string{
		".",
		"~/.config/SickRock/",
		"/config/",
	}, "SickRock.env")

	if err != nil {
		log.Warnf("Could not find env file: %v", err)
		return
	} else {
		log.Infof("Using env file: %s", envFile)
	}

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
			FullTimestamp: false,
		})
	}
}

func findFrontendDir() string {
	// Try to find the frontend directory
	possiblePaths := []string{
		"../frontend/",
		"frontend/",
		"/www/",
		"/usr/share/SickRock/frontend/",
		"/var/www/html/SickRock/frontend/",
	}

	indexHtml, _ := dirs.GetFirstExistingFileFromDirs("frontend", possiblePaths, "index.html")

	frontendDir := filepath.Dir(indexHtml)

	log.Infof("Using frontend directory: %s", frontendDir)

	return frontendDir
}

func main() {
	log.Info("SickRock is starting up...")

	loadEnvFile()

	configureLogging()

	db, err := repo.ConnectDatabase("file:../tmp/sickrock.db?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	log.Infof("Connected to database: %s", db.DriverName())

	repo := repo.NewRepository(db)

	if err := repo.EnsureSchema(context.Background()); err != nil {
		log.Fatalf("schema: %v", err)
	}

	srv := srvpkg.NewSickRockServer(repo)

	authService := auth.NewAuthService()

	interceptors := connect.WithInterceptors(auth.ConnectAuthMiddleware(authService))
	path, handler := sickrockpbconnect.NewSickRockHandler(srv, interceptors)

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginLogrusLogger())
	router.Use(gin.Recovery())
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

	router.Run(":" + getPort())
}

func getPort() string {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	log.Infof("Listening on port %s", port)

	return port
}
