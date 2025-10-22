package main

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/gin-gonic/gin"
	"github.com/jamesread/golure/pkg/dirs"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	"github.com/jamesread/SickRock/internal/auth"
	"github.com/jamesread/SickRock/internal/buildinfo"
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
	log.WithFields(log.Fields{
		"version": buildinfo.Version,
		"commit":  buildinfo.Commit,
		"date":    buildinfo.Date,
	}).Info("Build info")

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

	// Log database engine version before migrations
	logDatabaseEngineVersion(db)

	if err := runMigrations(db); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	// Log database engine version after migrations
	logDatabaseEngineVersion(db)

	// Reset admin password if environment variable is set
	if os.Getenv("SICKROCK_RESET_ADMIN_PASSWORD") != "" {
		log.Info("SICKROCK_RESET_ADMIN_PASSWORD environment variable is set, resetting admin password to 'admin'")
		if err := repo.UpdateUserPassword(context.Background(), "admin", "admin"); err != nil {
			log.Warnf("Failed to reset admin password: %v", err)
		} else {
			log.Info("Admin password has been reset to 'admin'")
		}
	}

	// Create default admin user if no users exist
	hasUsers, err := repo.HasUsers(context.Background())
	if err != nil {
		log.Fatalf("failed to check for existing users: %v", err)
	}
	if !hasUsers {
		log.Info("No users found in database, creating default admin user")
		if err := repo.CreateDefaultAdminUser(context.Background()); err != nil {
			log.Fatalf("failed to create default admin user: %v", err)
		}
		log.Info("Default admin user created (username: admin, password: admin)")
	}

	srv := srvpkg.NewSickRockServer(repo)

	authService := auth.NewAuthService(repo)

	// Start session cleanup job
	go startSessionCleanupJob(repo)

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

func runMigrations(db *sqlx.DB) error {
	// Use the underlying *sql.DB for migrate drivers
	sqlDB := db.DB

	driverName := db.DriverName()

	// Select migrations directory by driver
	cwd, _ := os.Getwd()
	var migDir string
	var databaseName string
	var d database.Driver

	switch driverName {
	case "mysql":
		migDir = filepath.Join(cwd, "migrations", "mysql")

		log.Infof("MySQL detected - migrations dir: %s", migDir)
		databaseName = "mysql"
		md, err := mysql.WithInstance(sqlDB, &mysql.Config{})
		if err != nil {
			return err
		}
		d = md
	default: // sqlite
		migDir = filepath.Join(cwd, "migrations", "sqlite")
		databaseName = "sqlite3"
		sd, err := sqlite3.WithInstance(sqlDB, &sqlite3.Config{})
		if err != nil {
			return err
		}
		d = sd
	}

	srcURL := "file://" + migDir
	m, err := migrate.NewWithDatabaseInstance(srcURL, databaseName, d)
	if err != nil {
		return err
	}
	// Do not close m here; Close() would close the shared *sql.DB instance

	// Version before
	beforeVer, beforeDirty, verr := m.Version()
	if verr == migrate.ErrNilVersion {
		beforeVer, beforeDirty = 0, false
		log.Infof("Migration version before: none (version=0), dirty=%v", beforeDirty)
	} else if verr != nil {
		log.Warnf("Could not get migration version before: %v", verr)
	} else {
		log.Infof("Migration version before: %d, dirty=%v", beforeVer, beforeDirty)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	// Version after
	afterVer, afterDirty, aerr := m.Version()
	if aerr == migrate.ErrNilVersion {
		afterVer, afterDirty = 0, false
		log.Infof("Migration version after: none (version=0), dirty=%v", afterDirty)
	} else if aerr != nil {
		log.Warnf("Could not get migration version after: %v", aerr)
	} else {
		log.Infof("Migration version after: %d, dirty=%v", afterVer, afterDirty)
	}

	log.Infof("Database migrations applied from %s", srcURL)
	return nil
}

func logDatabaseEngineVersion(db *sqlx.DB) {
	driver := db.DriverName()
	var version string
	var err error
	switch driver {
	case "mysql":
		err = db.Get(&version, "SELECT VERSION()")
	default: // sqlite3
		err = db.Get(&version, "SELECT sqlite_version()")
	}
	if err != nil {
		log.Warnf("Could not read database engine version: %v", err)
		return
	}
	log.Infof("Database engine %s version: %s", driver, version)
}

func getPort() string {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	log.Infof("Listening on port %s", port)

	return port
}

func startSessionCleanupJob(repo *repo.Repository) {
	ticker := time.NewTicker(7 * 24 * time.Hour) // Weekly cleanup
	defer ticker.Stop()

	log.Info("Session cleanup job started - will run weekly")

	// Run immediately on startup
	cleanupSessions(repo)

	// Then run weekly
	for range ticker.C {
		cleanupSessions(repo)
	}
}

func cleanupSessions(repo *repo.Repository) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := repo.CleanupExpiredSessions(ctx)
	if err != nil {
		log.Errorf("Session cleanup failed: %v", err)
	}

	err = repo.CleanupExpiredDeviceCodes(ctx)
	if err != nil {
		log.Errorf("Device code cleanup failed: %v", err)
	}
}
