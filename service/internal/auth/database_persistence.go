package auth

import (
	"context"

	"github.com/jamesread/httpauthshim/sessions"
	"github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

// DatabasePersistence implements SessionPersistence using the database backend.
// This allows httpauthshim to use our database sessions instead of YAML files.
type DatabasePersistence struct {
	repo *repo.Repository
}

// NewDatabasePersistence creates a new database-backed persistence backend.
func NewDatabasePersistence(repository *repo.Repository) *DatabasePersistence {
	return &DatabasePersistence{
		repo: repository,
	}
}

// Load loads sessions from the database into the SessionStorage.
// Since we manage sessions directly in the database, we don't need to load them
// into httpauthshim's in-memory storage - our provider checks the database directly.
func (p *DatabasePersistence) Load(dir, filename string, storage *sessions.SessionStorage) error {
	// We don't load sessions into httpauthshim's storage because:
	// 1. Our DatabaseAuthProvider checks the database directly
	// 2. We don't want to duplicate session data in memory
	// 3. The database is the source of truth
	log.Debug("DatabasePersistence.Load called - sessions are managed in database, not loading into memory")
	return nil
}

// Save saves sessions from SessionStorage to the database.
// Since we manage sessions directly in the database, we don't need to save from
// httpauthshim's in-memory storage - sessions are already in the database.
func (p *DatabasePersistence) Save(dir, filename string, storage *sessions.SessionStorage) error {
	// We don't save sessions from httpauthshim's storage because:
	// 1. Sessions are created directly in the database via AuthService.Login()
	// 2. We don't want to duplicate session management logic
	// 3. The database is the source of truth
	log.Debug("DatabasePersistence.Save called - sessions are managed in database, not saving from memory")
	return nil
}

// RequiresFileLock returns false since we use database storage, not file-based storage.
func (p *DatabasePersistence) RequiresFileLock() bool {
	return false
}

// SyncDatabaseSessionsToStorage is a helper method that can sync database sessions
// to httpauthshim's storage if needed (currently not used, but available for future use).
func (p *DatabasePersistence) SyncDatabaseSessionsToStorage(ctx context.Context, storage *sessions.SessionStorage, provider string) error {
	// This could be used to sync database sessions to httpauthshim's in-memory storage
	// if we ever need httpauthshim's session management features.
	// For now, we don't use this since our provider checks the database directly.

	// Example implementation (not currently used):
	// sessions, err := p.repo.GetAllActiveSessions(ctx)
	// if err != nil {
	//     return fmt.Errorf("failed to get sessions from database: %w", err)
	// }
	//
	// for _, sess := range sessions {
	//     storage.RegisterSession("", "", provider, sess.SessionID, sess.Username)
	// }

	return nil
}
