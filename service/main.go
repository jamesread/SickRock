package main

import (
	"bufio"
	"context"
	_ "embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesread/golure/pkg/dirs"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"

	connectproto "go.akshayshah.org/connectproto"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	armstore "github.com/jamesread/armature-iam/store"
	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	"github.com/jamesread/SickRock/internal/buildinfo"
	"github.com/jamesread/SickRock/internal/config"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/mcp"
	repo "github.com/jamesread/SickRock/internal/repo"
	srvpkg "github.com/jamesread/SickRock/internal/server"
)

//go:embed gen/openapi.json
var openAPISpec []byte

//go:embed llms.txt
var llmsTxt []byte

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

func loadEnvFile(cfg *config.Config) {
	searchDirs := []string{".", "~/.config/SickRock/", "/config/"}
	if cfg != nil && cfg.ConfigDir != "" {
		searchDirs = append([]string{cfg.ConfigDir}, searchDirs...)
	}
	envFile, err := dirs.GetFirstExistingFileFromDirs("env", searchDirs, "SickRock.env")

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

func configureLogging(cfg *config.Config) {
	level := cfg.LogLevel
	logLevel, err := log.ParseLevel(level)
	if err != nil {
		logLevel = log.InfoLevel
	}
	log.SetLevel(logLevel)

	if cfg.LogFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: false,
		})
	}
}

func findFrontendDir() string {
	if dir := os.Getenv("SICKROCK_FRONTEND_DIR"); dir != "" {
		log.Infof("Using frontend directory from SICKROCK_FRONTEND_DIR: %s", dir)
		return dir
	}

	possiblePaths := []string{
		"../frontend/dist/",
		"frontend/dist/",
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

func bootstrapIAM(ctx context.Context, iamStore armstore.Store) error {
	if err := iamStore.EnsureRBACBootstrap(ctx); err != nil {
		return err
	}

	if os.Getenv("SICKROCK_RESET_ADMIN_PASSWORD") != "" {
		log.Info("SICKROCK_RESET_ADMIN_PASSWORD is set, resetting admin password to 'admin'")
		user, err := iamStore.GetUserByUsername(ctx, "admin")
		if err != nil {
			return err
		}
		if user != nil {
			hash, err := iam.HashPassword("admin")
			if err != nil {
				return err
			}
			if err := iamStore.UpdateUserPassword(ctx, user.ID, hash); err != nil {
				log.Warnf("Failed to reset admin password: %v", err)
			} else {
				log.Info("Admin password has been reset to 'admin'")
			}
		}
	}

	count, err := iamStore.CountUserAccounts(ctx)
	if err != nil {
		return err
	}
	if count == 0 {
		log.Info("No users found, creating default admin user")
		hash, err := iam.HashPassword("admin")
		if err != nil {
			return err
		}
		if _, err := iamStore.CreateUserAccount(ctx, "admin", hash, armstore.UserCreatedByAdmin); err != nil {
			return err
		}
		if err := iamStore.EnsureRBACBootstrap(ctx); err != nil {
			return err
		}
		log.Info("Default admin user created (username: admin, password: admin)")
	}

	return nil
}

func main() {
	log.Info("SickRock is starting up...")
	log.WithFields(log.Fields{
		"version": buildinfo.Version,
		"commit":  buildinfo.Commit,
		"date":    buildinfo.Date,
	}).Info("Build info")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	loadEnvFile(cfg)
	configureLogging(cfg)

	db, err := repo.ConnectDatabase("file:../tmp/sickrock.db?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	log.Infof("Connected to database: %s", db.DriverName())

	repository := repo.NewRepository(db)

	logDatabaseEngineVersion(db)

	if err := runMigrations(db); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	if err := repository.UpdateSystemTableConfigurations(context.Background()); err != nil {
		log.Fatalf("update system table configurations: %v", err)
	}

	logDatabaseEngineVersion(db)

	iamStore := iam.NewStore(db)
	if err := bootstrapIAM(context.Background(), iamStore); err != nil {
		log.Fatalf("IAM bootstrap failed: %v", err)
	}

	authLayer, err := iam.NewAuthLayer(iamStore)
	if err != nil {
		log.Fatalf("auth layer: %v", err)
	}

	srv := srvpkg.NewSickRockServer(repository, authLayer)

	go startDeviceCodeCleanupJob(repository)

	jsonOpt := connectproto.WithJSON(
		protojson.MarshalOptions{
			EmitUnpopulated:   true,
			EmitDefaultValues: true,
			UseProtoNames:     false,
		},
		protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	)
	path, handler := sickrockpbconnect.NewSickRockHandler(srv, jsonOpt)
	handler = authLayer.WrapHandler(handler)

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginLogrusLogger())
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("Access-Control-Allow-Origin", "*")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		headers.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, Accept, connect-protocol-version, Session-Token")
		headers.Set("Access-Control-Allow-Credentials", "true")
		headers.Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
	router.Any("/api/*any", gin.WrapH(http.StripPrefix("/api", mux)))

	router.GET("/openapi", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Data(http.StatusOK, "application/json", openAPISpec)
	})

	router.GET("/llms.txt", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", llmsTxt)
	})

	mcpHandler := mcp.NewHandler(srv)
	router.Any("/mcp", gin.WrapH(authLayer.WrapMCPHandler(mcpHandler)))
	router.Any("/mcp/*path", gin.WrapH(authLayer.WrapMCPHandler(mcpHandler)))

	frontendDir := findFrontendDir()
	router.Static("/assets", filepath.Join(frontendDir, "assets"))
	router.Static("/css", filepath.Join(frontendDir, "css"))
	router.Static("/js", filepath.Join(frontendDir, "js"))
	router.Static("/images", filepath.Join(frontendDir, "images"))
	router.StaticFile("/favicon.ico", filepath.Join(frontendDir, "favicon.ico"))

	router.GET("/manifest.json", func(c *gin.Context) {
		manifestPath := filepath.Join(frontendDir, "manifest.json")
		c.Header("Content-Type", "application/manifest+json")
		c.File(manifestPath)
	})

	router.GET("/sw.js", func(c *gin.Context) {
		swPath := filepath.Join(frontendDir, "sw.js")
		c.Header("Content-Type", "application/javascript")
		c.Header("Service-Worker-Allowed", "/")
		c.File(swPath)
	})

	router.Static("/icons", filepath.Join(frontendDir, "icons"))
	router.StaticFile("/offline.html", filepath.Join(frontendDir, "offline.html"))
	router.Static("/screenshots", filepath.Join(frontendDir, "screenshots"))

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Status(http.StatusNotFound)
			return
		}
		c.File(filepath.Join(frontendDir, "index.html"))
	})

	addr := ":" + cfg.Port
	logListenPort(cfg.Port)
	if err := router.Run(addr); err != nil {
		log.WithError(err).WithField("addr", addr).Errorf("failed to listen or serve HTTP: %v", err)
		os.Exit(1)
	}
}

func runMigrations(db *sqlx.DB) error {
	sqlDB := db.DB
	driverName := db.DriverName()

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
	default:
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
	default:
		err = db.Get(&version, "SELECT sqlite_version()")
	}
	if err != nil {
		log.Warnf("Could not read database engine version: %v", err)
		return
	}
	log.Infof("Database engine %s version: %s", driver, version)
}

func logListenPort(port string) {
	log.Infof("Listening on port %s", port)
}

func startDeviceCodeCleanupJob(repository *repo.Repository) {
	ticker := time.NewTicker(7 * 24 * time.Hour)
	defer ticker.Stop()

	log.Info("Device code cleanup job started - will run weekly")
	cleanupDeviceCodes(repository)

	for range ticker.C {
		cleanupDeviceCodes(repository)
	}
}

func cleanupDeviceCodes(repository *repo.Repository) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := repository.CleanupExpiredDeviceCodes(ctx); err != nil {
		log.Errorf("Device code cleanup failed: %v", err)
	}
}
