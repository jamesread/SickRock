package main

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
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

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	"github.com/jamesread/SickRock/internal/buildinfo"
	"github.com/jamesread/SickRock/internal/config"
	"github.com/jamesread/SickRock/internal/health"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/mcp"
	repo "github.com/jamesread/SickRock/internal/repo"
	srvpkg "github.com/jamesread/SickRock/internal/server"
	armhealth "github.com/jamesread/armature-health"
	"github.com/jamesread/armature-health/errorlog"
	"github.com/jamesread/armature-health/store/sqlstore"
	armstore "github.com/jamesread/armature-iam/store"
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
	}
	log.Infof("Using env file: %s", envFile)

	if _, err := os.Stat(envFile); err == nil {
		applyEnvFile(envFile)
	}
}

// applyEnvFile sets environment variables from KEY=value lines, skipping blanks and # comments.
func applyEnvFile(envFile string) {
	file, err := os.Open(filepath.Clean(envFile))
	if err != nil {
		log.Warnf("Could not open .env file: %v", err)
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			_ = os.Setenv(key, value)
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
		if err := resetAdminPassword(ctx, iamStore); err != nil {
			return err
		}
	}

	return ensureDefaultAdmin(ctx, iamStore)
}

// resetAdminPassword sets the admin user's password back to "admin", if that user exists.
func resetAdminPassword(ctx context.Context, iamStore armstore.Store) error {
	log.Info("SICKROCK_RESET_ADMIN_PASSWORD is set, resetting admin password to 'admin'")
	user, err := iamStore.GetUserByUsername(ctx, "admin")
	if err != nil || user == nil {
		return err
	}
	hash, err := iam.HashPassword("admin")
	if err != nil {
		return err
	}
	if err := iamStore.UpdateUserPassword(ctx, user.ID, hash); err != nil {
		log.Warnf("Failed to reset admin password: %v", err)
	} else {
		log.Info("Admin password has been reset to 'admin'")
	}
	return nil
}

// ensureDefaultAdmin creates an admin/admin user when there are no user accounts at all.
func ensureDefaultAdmin(ctx context.Context, iamStore armstore.Store) error {
	count, err := iamStore.CountUserAccounts(ctx)
	if err != nil || count > 0 {
		return err
	}
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

	// Capture logged errors for the logs.recent_errors health check.
	errorLog := errorlog.New()
	log.AddHook(errorLog)

	db, repository := openDatabase()

	iamStore := iam.NewStore(db)
	if err = bootstrapIAM(context.Background(), iamStore); err != nil {
		log.Fatalf("IAM bootstrap failed: %v", err)
	}

	authLayer, err := iam.NewAuthLayer(iamStore)
	if err != nil {
		log.Fatalf("auth layer: %v", err)
	}

	srv := srvpkg.NewSickRockServer(repository, authLayer)

	go startDeviceCodeCleanupJob(repository)
	go startRssCalendarFeedRefreshJob(srv)
	startHealthCheckJob(cfg, repository, iamStore, srv, errorLog)

	router := newRouter(srv, authLayer)

	addr := ":" + cfg.Port
	logListenPort(cfg.Port)
	if err := router.Run(addr); err != nil {
		log.WithError(err).WithField("addr", addr).Errorf("failed to listen or serve HTTP: %v", err)
		os.Exit(1)
	}
}

// openDatabase connects to the database, applies migrations and refreshes system metadata, exiting on failure.
func openDatabase() (*sqlx.DB, *repo.Repository) {
	db, err := repo.ConnectDatabase("file:../tmp/sickrock.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	log.Infof("Connected to database: %s", db.DriverName())

	repository := repo.NewRepository(db)

	logDatabaseEngineVersion(db)

	if err = runMigrations(db); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	if err = repository.UpdateSystemTableConfigurations(context.Background()); err != nil {
		log.Fatalf("update system table configurations: %v", err)
	}

	if err = repository.BackfillTableRelationsFromPhysicalFKs(context.Background()); err != nil {
		log.Fatalf("backfill table_relations: %v", err)
	}

	logDatabaseEngineVersion(db)
	return db, repository
}

// newRouter serves the Connect API under /api, the AI discovery endpoints, MCP, metrics and the frontend.
func newRouter(srv *srvpkg.SickRockServer, authLayer *iam.AuthLayer) *gin.Engine {
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
	router.Use(corsMiddleware)
	router.Any("/api/*any", gin.WrapH(http.StripPrefix("/api", mux)))

	router.GET("/openapi", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Data(http.StatusOK, "application/json", openAPISpec)
	})

	router.GET("/llms.txt", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", llmsTxt)
	})

	router.GET("/metrics", gin.WrapH(metricsHandler()))

	mcpHandler := mcp.NewHandler(srv)
	router.Any("/mcp", gin.WrapH(authLayer.WrapMCPHandler(mcpHandler)))
	router.Any("/mcp/*path", gin.WrapH(authLayer.WrapMCPHandler(mcpHandler)))

	registerFrontendRoutes(router, findFrontendDir())
	return router
}

func corsMiddleware(c *gin.Context) {
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
}

// registerFrontendRoutes serves the built SPA from frontendDir, falling back to index.html for client routes.
func registerFrontendRoutes(router *gin.Engine, frontendDir string) {
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
		// Missing static assets (e.g. an unknown /assets/hugeicons/<version>/<Name>.json)
		// must 404 rather than fall back to the SPA page.
		if strings.HasPrefix(c.Request.URL.Path, "/api") || strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Status(http.StatusNotFound)
			return
		}
		c.File(filepath.Join(frontendDir, "index.html"))
	})
}

func runMigrations(db *sqlx.DB) error {
	cwd, _ := os.Getwd()
	migDir, databaseName, d, err := migrationDriver(db, cwd)
	if err != nil {
		return err
	}

	srcURL := "file://" + migDir
	m, err := migrate.NewWithDatabaseInstance(srcURL, databaseName, d)
	if err != nil {
		return err
	}

	logMigrationVersion(m, "before")

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	logMigrationVersion(m, "after")

	log.Infof("Database migrations applied from %s", srcURL)
	return nil
}

// migrationDriver returns the migrations directory, migrate database name and driver for db's engine.
func migrationDriver(db *sqlx.DB, cwd string) (migDir, databaseName string, d database.Driver, err error) {
	if db.DriverName() == "mysql" {
		migDir = filepath.Join(cwd, "migrations", "mysql")
		log.Infof("MySQL detected - migrations dir: %s", migDir)
		d, err = mysql.WithInstance(db.DB, &mysql.Config{})
		return migDir, "mysql", d, err
	}
	d, err = sqlite3.WithInstance(db.DB, &sqlite3.Config{})
	return filepath.Join(cwd, "migrations", "sqlite"), "sqlite3", d, err
}

func logMigrationVersion(m *migrate.Migrate, when string) {
	ver, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		log.Infof("Migration version %s: none (version=0), dirty=%v", when, dirty)
	case err != nil:
		log.Warnf("Could not get migration version %s: %v", when, err)
	default:
		log.Infof("Migration version %s: %d, dirty=%v", when, ver, dirty)
	}
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

// startHealthCheckJob creates the health check service, attaches it to the server and runs it
// in the background (shortly after startup, then every cfg.HealthCheckInterval).
func startHealthCheckJob(cfg *config.Config, repository *repo.Repository, iamStore armstore.Store, srv *srvpkg.SickRockServer, errorLog *errorlog.ErrorLog) {
	interval := healthCheckInterval(cfg.HealthCheckInterval)

	checks := health.DefaultChecks(health.Deps{
		Repo:     repository,
		ErrorLog: errorLog,
		AdminUsesDefaultPassword: func(ctx context.Context) (bool, error) {
			return adminUsesDefaultPassword(ctx, iamStore)
		},
		Getenv:   os.Getenv,
		LogLevel: cfg.LogLevel,
	})
	db := repository.DB()
	svc := armhealth.NewService(sqlstore.New(db.DB, db.DriverName()), checks, srv.NotificationService(), armhealth.Options{})
	srv.SetHealthService(svc)
	go svc.RunPeriodically(context.Background(), time.Minute, interval)
}

func healthCheckInterval(raw string) time.Duration {
	interval, err := time.ParseDuration(raw)
	if err != nil || interval < time.Minute {
		log.Warnf("Invalid health_check_interval %q (minimum 1m), using 15m", raw)
		return 15 * time.Minute
	}
	return interval
}

func adminUsesDefaultPassword(ctx context.Context, iamStore armstore.Store) (bool, error) {
	user, err := iamStore.GetUserByUsername(ctx, "admin")
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}
	return iam.VerifyPassword(user.PasswordHash, "admin")
}

func startRssCalendarFeedRefreshJob(srv *srvpkg.SickRockServer) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Info("RSS calendar feed refresh job started (checks every 5 minutes, default feed interval 60 minutes)")
	refreshDueRssFeeds(srv)

	for range ticker.C {
		refreshDueRssFeeds(srv)
	}
}

func refreshDueRssFeeds(srv *srvpkg.SickRockServer) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	srv.RefreshDueRssCalendarFeeds(ctx)
}
