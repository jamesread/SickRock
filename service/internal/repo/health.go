package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// HealthIssue is a stored issue found by a health check (see internal/health). Times are unix seconds.
type HealthIssue struct {
	ID             int64         `db:"id"`
	CheckID        string        `db:"check_id"`
	IssueKey       string        `db:"issue_key"`
	Severity       string        `db:"severity"`
	Title          string        `db:"title"`
	Detail         string        `db:"detail"`
	Link           string        `db:"link"`
	FirstSeen      int64         `db:"first_seen"`
	LastSeen       int64         `db:"last_seen"`
	Occurrences    int           `db:"occurrences"`
	ResolvedAt     sql.NullInt64 `db:"resolved_at"`
	AcknowledgedAt sql.NullInt64 `db:"acknowledged_at"`
	AcknowledgedBy sql.NullInt64 `db:"acknowledged_by"`
}

// HealthCheckRun is the last run of a health check.
type HealthCheckRun struct {
	CheckID    string `db:"check_id"`
	LastRunAt  int64  `db:"last_run_at"`
	DurationMs int64  `db:"duration_ms"`
	Status     string `db:"status"`
	Error      string `db:"error"`
}

// HealthSummary counts open, unacknowledged issues by severity.
type HealthSummary struct {
	Errors   int
	Warnings int
	Infos    int
}

const healthIssueColumns = `id, check_id, issue_key, severity, title, detail, link, first_seen, last_seen,
	occurrences, resolved_at, acknowledged_at, acknowledged_by`

// ListOpenHealthIssuesForCheck returns a check's unresolved issues.
func (r *Repository) ListOpenHealthIssuesForCheck(ctx context.Context, checkID string) ([]HealthIssue, error) {
	var issues []HealthIssue
	err := r.db.SelectContext(ctx, &issues,
		"SELECT "+healthIssueColumns+" FROM health_issues WHERE check_id = ? AND resolved_at IS NULL", checkID)
	return issues, err
}

// InsertHealthIssue stores a newly found issue and returns its id.
func (r *Repository) InsertHealthIssue(ctx context.Context, issue HealthIssue) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO health_issues (check_id, issue_key, severity, title, detail, link, first_seen, last_seen, occurrences)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue.CheckID, issue.IssueKey, issue.Severity, issue.Title, issue.Detail, issue.Link,
		issue.FirstSeen, issue.LastSeen, issue.Occurrences)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateSeenHealthIssue refreshes an open issue that a check found again.
func (r *Repository) UpdateSeenHealthIssue(ctx context.Context, issue HealthIssue) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE health_issues SET severity = ?, title = ?, detail = ?, link = ?, last_seen = ?, occurrences = ?
		WHERE id = ?`,
		issue.Severity, issue.Title, issue.Detail, issue.Link, issue.LastSeen, issue.Occurrences, issue.ID)
	return err
}

// ResolveHealthIssue marks an open issue as resolved.
func (r *Repository) ResolveHealthIssue(ctx context.Context, id int64, at int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE health_issues SET resolved_at = ? WHERE id = ? AND resolved_at IS NULL", at, id)
	return err
}

// ListHealthIssues returns open issues (optionally with acknowledged and resolved ones), newest first.
func (r *Repository) ListHealthIssues(ctx context.Context, includeResolved, includeAcknowledged bool) ([]HealthIssue, error) {
	var where []string
	if !includeResolved {
		where = append(where, "resolved_at IS NULL")
	}
	if !includeAcknowledged {
		where = append(where, "acknowledged_at IS NULL")
	}
	query := "SELECT " + healthIssueColumns + " FROM health_issues"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY last_seen DESC, id DESC"

	var issues []HealthIssue
	err := r.db.SelectContext(ctx, &issues, query)
	return issues, err
}

// SetHealthIssueAcknowledged acknowledges (or un-acknowledges) an issue.
func (r *Repository) SetHealthIssueAcknowledged(ctx context.Context, id int64, acknowledged bool, userID int, at int64) error {
	var res sql.Result
	var err error
	if acknowledged {
		res, err = r.db.ExecContext(ctx, "UPDATE health_issues SET acknowledged_at = ?, acknowledged_by = ? WHERE id = ?", at, userID, id)
	} else {
		res, err = r.db.ExecContext(ctx, "UPDATE health_issues SET acknowledged_at = NULL, acknowledged_by = NULL WHERE id = ?", id)
	}
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("health issue %d not found", id)
	}
	return nil
}

// GetHealthSummary counts open, unacknowledged issues by severity.
func (r *Repository) GetHealthSummary(ctx context.Context) (HealthSummary, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT severity, COUNT(*) FROM health_issues
		WHERE resolved_at IS NULL AND acknowledged_at IS NULL
		GROUP BY severity`)
	if err != nil {
		return HealthSummary{}, err
	}
	defer rows.Close()

	var summary HealthSummary
	for rows.Next() {
		var severity string
		var count int
		if err := rows.Scan(&severity, &count); err != nil {
			return HealthSummary{}, err
		}
		switch severity {
		case "error":
			summary.Errors = count
		case "warning":
			summary.Warnings = count
		default:
			summary.Infos += count
		}
	}
	return summary, rows.Err()
}

// PruneResolvedHealthIssues deletes issues resolved before the given time.
func (r *Repository) PruneResolvedHealthIssues(ctx context.Context, before int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM health_issues WHERE resolved_at IS NOT NULL AND resolved_at < ?", before)
	return err
}

// SaveHealthCheckRun records the outcome of a check's latest run.
func (r *Repository) SaveHealthCheckRun(ctx context.Context, run HealthCheckRun) error {
	var query string
	switch r.db.DriverName() {
	case "mysql":
		query = `INSERT INTO health_check_runs (check_id, last_run_at, duration_ms, status, error) VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE last_run_at = VALUES(last_run_at), duration_ms = VALUES(duration_ms),
				status = VALUES(status), error = VALUES(error)`
	default:
		query = `INSERT INTO health_check_runs (check_id, last_run_at, duration_ms, status, error) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(check_id) DO UPDATE SET last_run_at = excluded.last_run_at, duration_ms = excluded.duration_ms,
				status = excluded.status, error = excluded.error`
	}
	_, err := r.db.ExecContext(ctx, query, run.CheckID, run.LastRunAt, run.DurationMs, run.Status, run.Error)
	return err
}

// ListHealthCheckRuns returns the latest run of every check that has run.
func (r *Repository) ListHealthCheckRuns(ctx context.Context) ([]HealthCheckRun, error) {
	var runs []HealthCheckRun
	err := r.db.SelectContext(ctx, &runs, "SELECT check_id, last_run_at, duration_ms, status, error FROM health_check_runs")
	return runs, err
}
