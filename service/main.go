package main

import (
	"context"

	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	repo "github.com/jamesread/SickRock/internal/repo"
	srvpkg "github.com/jamesread/SickRock/internal/server"
	_ "modernc.org/sqlite"
)

func main() {
	log.Info("SickRock is starting up...")

	router := gin.Default()

	// Serve built frontend (Vite dist)
	distDir := filepath.Join("..", "frontend", "dist")
	router.Static("/assets", filepath.Join(distDir, "assets"))
	router.Static("/resources", filepath.Join(distDir, "resources"))
	router.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(distDir, "index.html"))
	})

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

	_ = r.InsertTableConfiguration(context.Background(), "computers")
	_ = r.InsertTableConfiguration(context.Background(), "contacts")

	// Seed dummy data for demo tables if empty
	seed := map[string][]string{
		"computers": {"MacBook Pro", "ThinkPad X1"},
		"contacts":  {"Alice Johnson", "Bob Smith"},
	}
	for table, names := range seed {
		if err := r.EnsureSchemaForTable(context.Background(), table); err != nil {
			log.Fatalf("ensure schema %s: %v", table, err)
		}
		existing, err := r.ListItemsInTable(context.Background(), table)
		if err != nil {
			log.Fatalf("list %s: %v", table, err)
		}
		if len(existing) == 0 {
			for _, name := range names {
				if _, err := r.CreateItemInTable(context.Background(), table, name); err != nil {
					log.Warnf("seed %s: %v", table, err)
				}
			}
		}
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
		c.File(filepath.Join(distDir, "index.html"))
	})

	router.Run(":8080")
}
