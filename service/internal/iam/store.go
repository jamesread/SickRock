package iam

import (
	"github.com/jamesread/armature-iam/store"
	iamsqlite "github.com/jamesread/armature-iam/store/sqlite"
	"github.com/jamesread/SickRock/internal/iam/mysqlstore"
	"github.com/jmoiron/sqlx"
)

// NewStore returns an armature-iam store for the active database driver.
func NewStore(db *sqlx.DB) store.Store {
	switch db.DriverName() {
	case "mysql":
		return mysqlstore.NewMySQL(db.DB)
	default:
		return iamsqlite.New(db.DB)
	}
}
