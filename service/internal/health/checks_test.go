package health

import (
	"context"
	"strings"
	"testing"

	armhealth "github.com/jamesread/armature-health"
	"github.com/jamesread/armature-health/errorlog"
	"github.com/jmoiron/sqlx"

	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

// newBrokenDB returns a migrated database seeded with one of each problem the checks look for.
func newBrokenDB(t *testing.T) (*sqlx.DB, *repo.Repository) {
	t.Helper()
	db := testdb.NewMigratedSQLite(t)
	mustExec(t, db, `
		CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT);
		CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER);

		INSERT INTO table_configurations (id, name, db, "table") VALUES
			(901, 'customers', 'main', 'customers'),
			(902, 'orders', 'main', 'orders'),
			(903, 'ghost', 'main', 'no_such_table');

		INSERT INTO table_navigation (id, table_configuration, name, title) VALUES
			(801, 901, 'customers', 'Customers'),
			(802, NULL, 'empty', '');
		INSERT INTO table_navigation (id, dashboard_id, name) VALUES (803, 4242, 'old dashboard');

		INSERT INTO table_user_bookmarks (id, user, navigation_item) VALUES (701, 1, 801), (702, 1, 9999);

		INSERT INTO table_relations (constraint_name, source_table_key, source_column, referenced_table_key, referenced_column) VALUES
			('fk_ok', 'orders', 'customer_id', 'customers', 'id'),
			('fk_bad_column', 'orders', 'gone_column', 'customers', 'id'),
			('fk_bad_table', 'invoices', 'customer_id', 'customers', 'id');

		INSERT INTO rss_calendar_feeds (id, name, feed_url, table_configuration, enabled, last_refresh_status, last_refresh_message) VALUES
			(601, 'Scouts', 'https://example.com/a.ics', 'famcal', 1, 'error', 'Field ''starts'' doesn''t have a default value'),
			(602, 'Disabled', 'https://example.com/b.ics', 'famcal', 0, 'error', 'ignored'),
			(603, 'Fine', 'https://example.com/c.ics', 'famcal', 1, 'ok', '');
	`)
	return db, repo.NewRepository(db)
}

func mustExec(t *testing.T, db *sqlx.DB, query string) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), query); err != nil {
		t.Fatalf("fixture: %v", err)
	}
}

func runCheck(t *testing.T, check armhealth.Check) map[string]armhealth.Issue {
	t.Helper()
	issues, err := check.Run(context.Background())
	if err != nil {
		t.Fatalf("%s: %v", check.ID(), err)
	}
	byKey := map[string]armhealth.Issue{}
	for _, issue := range issues {
		byKey[issue.Key] = issue
	}
	return byKey
}

func expectKeys(t *testing.T, check armhealth.Check, got map[string]armhealth.Issue, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d issues %v, want keys %v", check.ID(), len(got), got, want)
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Fatalf("%s: missing issue %q in %v", check.ID(), key, got)
		}
	}
}

func TestChecksFindSeededProblems(t *testing.T) {
	_, r := newBrokenDB(t)

	dup := TableConfigurationDuplicateNamesCheck{Repo: r}
	expectKeys(t, dup, runCheck(t, dup))

	missing := TableConfigurationMissingTableCheck{Repo: r}
	got := runCheck(t, missing)
	expectKeys(t, missing, got, "tc:903")
	if got["tc:903"].Link != "/table/table_configurations/903/edit" {
		t.Fatalf("link %q", got["tc:903"].Link)
	}

	nav := NavigationBrokenLinksCheck{Repo: r}
	got = runCheck(t, nav)
	expectKeys(t, nav, got, "nav:802", "nav:803")
	if !strings.Contains(got["nav:803"].Title, "dashboard #4242") || !strings.Contains(got["nav:802"].Title, "does not point to anything") {
		t.Fatalf("navigation titles %+v", got)
	}

	bookmarks := BookmarksOrphanedCheck{Repo: r}
	expectKeys(t, bookmarks, runCheck(t, bookmarks), "bookmark:702")

	relations := RelationsDanglingCheck{Repo: r}
	got = runCheck(t, relations)
	expectKeys(t, relations, got, "relation:fk_bad_column", "relation:fk_bad_table")
	if !strings.Contains(got["relation:fk_bad_column"].Title, `"gone_column"`) || got["relation:fk_bad_column"].Link != "/table/orders/foreign-keys" {
		t.Fatalf("relation issue %+v", got["relation:fk_bad_column"])
	}

	feeds := CalendarFeedsFailingCheck{Repo: r}
	got = runCheck(t, feeds)
	expectKeys(t, feeds, got, "feed:601")
	if !strings.Contains(got["feed:601"].Detail, "default value") {
		t.Fatalf("feed detail %q", got["feed:601"].Detail)
	}
}

// Legacy schemas (like production databases created before the migrations) have no unique
// index on table_configurations.name, which is how duplicates appear.
func TestDuplicateTableConfigurationNames(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	mustExec(t, db, `
		PRAGMA foreign_keys = OFF;
		CREATE TABLE legacy_tc AS SELECT * FROM table_configurations;
		DROP TABLE table_configurations;
		ALTER TABLE legacy_tc RENAME TO table_configurations;
		INSERT INTO table_configurations (id, name) VALUES (901, 'table_settings'), (902, 'table_settings'), (903, 'unique_one');
	`)
	check := TableConfigurationDuplicateNamesCheck{Repo: repo.NewRepository(db)}
	got := runCheck(t, check)
	expectKeys(t, check, got, "name:table_settings")
	if issue := got["name:table_settings"]; issue.Count < 2 || issue.Severity != armhealth.SeverityError {
		t.Fatalf("duplicate issue %+v", issue)
	}
}

func TestRiskySettingsWiring(t *testing.T) {
	env := map[string]string{"SICKROCK_RESET_ADMIN_PASSWORD": "1"}
	var risky armhealth.Check
	for _, check := range DefaultChecks(Deps{Getenv: func(k string) string { return env[k] }, LogLevel: "debug"}) {
		if check.ID() == "config.risky_settings" {
			risky = check
		}
	}
	if risky == nil {
		t.Fatal("config.risky_settings not registered")
	}
	expectKeys(t, risky, runCheck(t, risky), "env:SICKROCK_RESET_ADMIN_PASSWORD", "log-level")
}

func TestDefaultChecksHaveUniqueIDs(t *testing.T) {
	checks := DefaultChecks(Deps{
		Repo:                     repo.NewRepository(testdb.NewMigratedSQLite(t)),
		ErrorLog:                 errorlog.New(),
		AdminUsesDefaultPassword: func(context.Context) (bool, error) { return false, nil },
		Getenv:                   func(string) string { return "" },
	})
	seen := map[string]bool{}
	for _, check := range checks {
		if seen[check.ID()] || check.Title() == "" {
			t.Fatalf("check %q is duplicated or untitled", check.ID())
		}
		seen[check.ID()] = true
	}
	if len(checks) != 9 {
		t.Fatalf("expected 9 default checks, got %d", len(checks))
	}
}
