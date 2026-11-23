package repo

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// RollupsTableConfiguration is the system table configuration that manages rollup definitions.
const RollupsTableConfiguration = "table_rollups"

// ErrRollupNotFound is returned when no rollup has the requested id.
var ErrRollupNotFound = errors.New("rollup not found")

// ErrInvalidRollup wraps problems with a rollup's definition or filter, such as unknown columns.
var ErrInvalidRollup = errors.New("invalid rollup")

// rollupLinkBatch is how many owner ids go in one IN (...) query for detail rows.
const rollupLinkBatch = 500

// Rollup gathers the detail rows owned by a set of source rows, groups them by GroupColumns, sums
// SumColumns, and replaces every row of TargetTable with one row per group. For example: the
// ingredients (detail) of the recipes linked from a week's meal plan rows (source), summed per
// ingredient and unit into a shopping list (target).
type Rollup struct {
	// ColumnMap maps a detail column to the target column it is written to (default: same name).
	ColumnMap map[string]string
	Name      string
	// SourceTable is the table configuration a run starts from.
	SourceTable string
	// SourceLinkColumn is the column of the source rows holding the owner id the detail rows
	// reference; empty when the detail rows reference the source rows themselves.
	SourceLinkColumn string
	DetailTable      string
	// DetailLinkColumn is the detail table's column referencing the owner.
	DetailLinkColumn string
	TargetTable      string
	// CountColumn, when set, is the target column given how many source rows fed each group.
	CountColumn  string
	GroupColumns []string
	SumColumns   []string
	ID           int64
}

// RollupSourceFilter narrows the source rows of a run.
type RollupSourceFilter struct {
	From time.Time
	To   time.Time
	// Where holds exact-match filters on source columns.
	Where map[string]string
	// DateColumn, when set, keeps source rows whose value lies in [From, To); a zero bound is open.
	DateColumn string
	// SourceIDs, when LimitToSourceIDs is set, are the primary keys of the only source rows used.
	SourceIDs        []string
	LimitToSourceIDs bool
}

// RollupResult reports what a run did.
type RollupResult struct {
	// Skipped describes values that could not be summed.
	Skipped     []string
	SourceRows  int
	DetailRows  int
	RowsWritten int
}

const rollupSelectColumns = `id, name, source_table, source_link_column, detail_table, detail_link_column,
	group_columns_json, sum_columns_json, target_table, column_map_json, count_column`

// GetRollup returns the rollup with id, or ErrRollupNotFound.
func (r *Repository) GetRollup(ctx context.Context, id int64) (*Rollup, error) {
	row := r.db.QueryRowxContext(ctx, `SELECT `+rollupSelectColumns+` FROM rollups WHERE id = ?`, id)
	ru, err := scanRollup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRollupNotFound
	}
	return ru, err
}

// ListRollups returns every rollup by name, or only those starting from sourceTable when it is set.
func (r *Repository) ListRollups(ctx context.Context, sourceTable string) ([]Rollup, error) {
	query := `SELECT ` + rollupSelectColumns + ` FROM rollups`
	var args []any
	if sourceTable != "" {
		query += ` WHERE source_table = ?`
		args = append(args, sourceTable)
	}
	rows, err := r.db.QueryxContext(ctx, query+` ORDER BY name, id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Rollup
	for rows.Next() {
		ru, err := scanRollup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ru)
	}
	return out, rows.Err()
}

func scanRollup(row interface{ Scan(dest ...any) error }) (*Rollup, error) {
	var ru Rollup
	var groupJSON, sumJSON, mapJSON string
	if err := row.Scan(&ru.ID, &ru.Name, &ru.SourceTable, &ru.SourceLinkColumn, &ru.DetailTable, &ru.DetailLinkColumn,
		&groupJSON, &sumJSON, &ru.TargetTable, &mapJSON, &ru.CountColumn); err != nil {
		return nil, err
	}
	if err := ru.parseJSONColumns(groupJSON, sumJSON, mapJSON); err != nil {
		return nil, err
	}
	ru.SourceLinkColumn = strings.TrimSpace(ru.SourceLinkColumn)
	ru.DetailLinkColumn = strings.TrimSpace(ru.DetailLinkColumn)
	ru.CountColumn = strings.TrimSpace(ru.CountColumn)
	return &ru, nil
}

func (ru *Rollup) parseJSONColumns(groupJSON, sumJSON, mapJSON string) error {
	var err error
	if ru.GroupColumns, err = parseRollupColumnList(groupJSON); err != nil {
		return fmt.Errorf("rollup %d group_columns_json: %w", ru.ID, err)
	}
	if ru.SumColumns, err = parseRollupColumnList(sumJSON); err != nil {
		return fmt.Errorf("rollup %d sum_columns_json: %w", ru.ID, err)
	}
	if ru.ColumnMap, err = parseRollupColumnMap(mapJSON); err != nil {
		return fmt.Errorf("rollup %d column_map_json: %w", ru.ID, err)
	}
	return nil
}

// unmarshalOptionalJSON decodes raw into v, leaving v alone when raw is blank.
func unmarshalOptionalJSON(raw string, v any) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), v)
}

// parseRollupColumnList reads a JSON array of column names, dropping blanks and repeats. Empty text is [].
func parseRollupColumnList(raw string) ([]string, error) {
	var list []string
	if err := unmarshalOptionalJSON(raw, &list); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, name := range list {
		if name = strings.TrimSpace(name); name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out, nil
}

// parseRollupColumnMap reads a JSON object of detail column -> target column, dropping blanks.
func parseRollupColumnMap(raw string) (map[string]string, error) {
	m := map[string]string{}
	if err := unmarshalOptionalJSON(raw, &m); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k, v = strings.TrimSpace(k), strings.TrimSpace(v); k != "" && v != "" {
			out[k] = v
		}
	}
	return out, nil
}

// rollupTable is one table a run reads or writes, with its columns.
type rollupTable struct {
	tc      *TableConfig
	ref     string
	pk      string
	columns []FieldSpec
}

func (r *Repository) loadRollupTable(ctx context.Context, tcName string) (*rollupTable, error) {
	tc, err := r.GetTableConfiguration(ctx, tcName)
	if err != nil {
		return nil, fmt.Errorf("table %q: %w", tcName, err)
	}
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return nil, fmt.Errorf("columns of table %q: %w", tcName, err)
	}
	return &rollupTable{
		tc:      tc,
		ref:     fmt.Sprintf("`%s`.`%s`", tc.Db.String, tc.Table.String),
		pk:      tc.PrimaryKeyColumnName(),
		columns: columns,
	}, nil
}

// column returns the real name of a column of the table, or an error naming what it is for.
func (t *rollupTable) column(name, role string) (string, error) {
	col := writableColumn(t.columns, name)
	if col == nil {
		return "", fmt.Errorf("%s %q is not a column of table %q", role, name, t.tc.Name)
	}
	return col.Name, nil
}

// resolveColumns resolves each name to a real column of the table.
func (t *rollupTable) resolveColumns(names []string, role string) ([]string, error) {
	out := make([]string, 0, len(names))
	for _, name := range names {
		col, err := t.column(name, role)
		if err != nil {
			return nil, err
		}
		out = append(out, col)
	}
	return out, nil
}

// optionalColumn resolves name like column, but an empty name stays empty.
func (t *rollupTable) optionalColumn(name, role string) (string, error) {
	if name == "" {
		return "", nil
	}
	return t.column(name, role)
}

// rollupPlan is a rollup with every column checked against the real tables.
type rollupPlan struct {
	source, detail, target *rollupTable
	where                  map[string]string
	// targetOf maps each group and sum column to its target column.
	targetOf     map[string]string
	sourceLink   string // "" when details reference the source key
	dateColumn   string
	detailLink   string
	countColumn  string
	groupColumns []string
	sumColumns   []string
}

// planRollup loads the rollup's tables and resolves every configured column name.
func (r *Repository) planRollup(ctx context.Context, ru *Rollup, filter RollupSourceFilter) (*rollupPlan, error) {
	if len(ru.GroupColumns) == 0 {
		return nil, errors.New("a rollup needs at least one group column")
	}
	p := &rollupPlan{targetOf: map[string]string{}}
	if err := r.loadRollupTables(ctx, ru, p); err != nil {
		return nil, err
	}
	for _, step := range []func() error{
		func() error { return checkRollupTarget(p) },
		func() error { return p.resolveSource(ru, filter) },
		func() error { return p.resolveDetail(ru) },
		func() error { return p.resolveTarget(ru) },
	} {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (r *Repository) loadRollupTables(ctx context.Context, ru *Rollup, p *rollupPlan) error {
	for _, t := range []struct {
		dst  **rollupTable
		name string
	}{{&p.source, ru.SourceTable}, {&p.detail, ru.DetailTable}, {&p.target, ru.TargetTable}} {
		table, err := r.loadRollupTable(ctx, t.name)
		if err != nil {
			return err
		}
		*t.dst = table
	}
	return nil
}

// checkRollupTarget refuses a target that a run would destroy by mistake: a system table or the
// rollup's own source or detail table.
func checkRollupTarget(p *rollupPlan) error {
	if IsSystemTableConfiguration(p.target.tc.Name) {
		return fmt.Errorf("target table %q is a system table", p.target.tc.Name)
	}
	if strings.EqualFold(p.source.ref, p.target.ref) || strings.EqualFold(p.detail.ref, p.target.ref) {
		return fmt.Errorf("target table %q is also the rollup's source or detail table", p.target.tc.Name)
	}
	return nil
}

func (p *rollupPlan) resolveSource(ru *Rollup, filter RollupSourceFilter) error {
	var err error
	if p.sourceLink, err = p.source.optionalColumn(ru.SourceLinkColumn, "source link column"); err != nil {
		return err
	}
	if p.dateColumn, err = p.source.optionalColumn(filter.DateColumn, "date column"); err != nil {
		return err
	}
	p.where, err = p.source.resolveWhere(filter.Where)
	return err
}

// resolveWhere keys exact-match filters by real column names.
func (t *rollupTable) resolveWhere(where map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(where))
	for key, value := range where {
		col, err := t.column(key, "filter column")
		if err != nil {
			return nil, err
		}
		out[col] = value
	}
	return out, nil
}

func (p *rollupPlan) resolveDetail(ru *Rollup) error {
	var err error
	if p.detailLink, err = p.detail.column(ru.DetailLinkColumn, "detail link column"); err != nil {
		return err
	}
	if p.groupColumns, err = p.detail.resolveColumns(ru.GroupColumns, "group column"); err != nil {
		return err
	}
	if p.sumColumns, err = p.detail.resolveColumns(ru.SumColumns, "sum column"); err != nil {
		return err
	}
	if i := slices.IndexFunc(p.sumColumns, func(c string) bool { return slices.Contains(p.groupColumns, c) }); i >= 0 {
		return fmt.Errorf("column %q cannot be both grouped and summed", p.sumColumns[i])
	}
	return nil
}

func (p *rollupPlan) resolveTarget(ru *Rollup) error {
	for _, col := range slices.Concat(p.groupColumns, p.sumColumns) {
		name := col
		if mapped, ok := mapLookupFold(ru.ColumnMap, col); ok {
			name = mapped
		}
		targetCol, err := p.target.column(name, "target column")
		if err != nil {
			return err
		}
		p.targetOf[col] = targetCol
	}
	var err error
	p.countColumn, err = p.target.optionalColumn(ru.CountColumn, "count column")
	return err
}

// mapLookupFold looks key up exactly, then case-insensitively.
func mapLookupFold(m map[string]string, key string) (string, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return "", false
}

// RunRollup replaces every row of the rollup's target table with the grouped and summed detail rows
// of the source rows matching filter, in one transaction. Each source row counts once, so an owner
// linked from two source rows contributes its details twice. The target's tick list state is cleared.
func (r *Repository) RunRollup(ctx context.Context, ru *Rollup, filter RollupSourceFilter) (RollupResult, error) {
	if ru == nil {
		return RollupResult{}, ErrRollupNotFound
	}
	p, err := r.planRollup(ctx, ru, filter)
	if err != nil {
		return RollupResult{}, fmt.Errorf("%w: %w", ErrInvalidRollup, err)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return RollupResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := r.runRollupTx(ctx, tx, p, filter)
	if err != nil {
		return RollupResult{}, err
	}
	return res, tx.Commit()
}

func (r *Repository) runRollupTx(ctx context.Context, tx *sqlx.Tx, p *rollupPlan, filter RollupSourceFilter) (RollupResult, error) {
	var res RollupResult
	owners, err := p.sourceOwners(ctx, tx, filter, &res)
	if err != nil {
		return res, fmt.Errorf("read source rows: %w", err)
	}
	groups, err := p.gatherGroups(ctx, tx, owners, &res)
	if err != nil {
		return res, fmt.Errorf("read detail rows: %w", err)
	}
	if err = r.replaceTargetRows(ctx, tx, p, groups); err != nil {
		return res, fmt.Errorf("write target rows: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM tick_list_state WHERE tc_name = ?`, p.target.tc.Name); err != nil {
		return res, fmt.Errorf("clear tick list state: %w", err)
	}
	res.RowsWritten = len(groups)
	return res, nil
}

// sourceOwners returns how many matching source rows link to each owner id.
func (p *rollupPlan) sourceOwners(ctx context.Context, tx *sqlx.Tx, filter RollupSourceFilter, res *RollupResult) (map[string]int, error) {
	cols := []string{cmp.Or(p.sourceLink, p.source.pk), p.source.pk, cmp.Or(p.dateColumn, p.source.pk)}
	whereSQL, args := equalityWhere(p.where)
	rows, err := tx.QueryxContext(ctx, fmt.Sprintf("SELECT %s FROM %s%s", quoteColumns(cols), p.source.ref, whereSQL), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	owners := map[string]int{}
	keep := p.sourceRowFilter(filter)
	for rows.Next() {
		values, err := rows.SliceScan()
		if err != nil {
			return nil, err
		}
		if keep(rollupValueString(values[1]), values[2]) {
			countSourceRow(rollupValueString(values[0]), owners, res)
		}
	}
	return owners, rows.Err()
}

// sourceRowFilter reports whether a source row (its key and date value) passes the filter's id
// list and date range.
func (p *rollupPlan) sourceRowFilter(filter RollupSourceFilter) func(id string, date any) bool {
	ids := sourceIDSet(filter)
	return func(id string, date any) bool {
		if ids != nil && !ids[id] {
			return false
		}
		return p.dateColumn == "" || inRollupRange(date, filter.From, filter.To)
	}
}

// sourceIDSet is the set of allowed source ids, or nil when the filter lists none.
func sourceIDSet(filter RollupSourceFilter) map[string]bool {
	if !filter.LimitToSourceIDs {
		return nil
	}
	ids := make(map[string]bool, len(filter.SourceIDs))
	for _, id := range filter.SourceIDs {
		ids[strings.TrimSpace(id)] = true
	}
	return ids
}

// countSourceRow counts a source row linking to owner.
func countSourceRow(owner string, owners map[string]int, res *RollupResult) {
	res.SourceRows++
	if owner != "" {
		owners[owner]++
	}
}

// rollupGroup is one output row being built.
type rollupGroup struct {
	seen    map[string]bool
	values  []string
	sums    []float64
	hasSum  []bool
	sources int
}

// gatherGroups reads the detail rows of owners and adds them into groups, sorted by group values.
func (p *rollupPlan) gatherGroups(ctx context.Context, tx *sqlx.Tx, owners map[string]int, res *RollupResult) ([]*rollupGroup, error) {
	byKey := map[string]*rollupGroup{}
	cols := slices.Concat([]string{p.detailLink}, p.groupColumns, p.sumColumns)
	for chunk := range slices.Chunk(slices.Sorted(maps.Keys(owners)), rollupLinkBatch) {
		query := fmt.Sprintf("SELECT %s FROM %s WHERE `%s` IN (%s)",
			quoteColumns(cols), p.detail.ref, p.detailLink, strings.TrimSuffix(strings.Repeat("?, ", len(chunk)), ", "))
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		if err := p.addDetailRows(ctx, tx, query, args, owners, byKey, res); err != nil {
			return nil, err
		}
	}
	groups := slices.Collect(maps.Values(byKey))
	slices.SortFunc(groups, func(a, b *rollupGroup) int {
		return slices.CompareFunc(a.values, b.values, func(x, y string) int {
			return strings.Compare(strings.ToLower(x), strings.ToLower(y))
		})
	})
	return groups, nil
}

func (p *rollupPlan) addDetailRows(ctx context.Context, tx *sqlx.Tx, query string, args []any, owners map[string]int, byKey map[string]*rollupGroup, res *RollupResult) error {
	rows, err := tx.QueryxContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		values, err := rows.SliceScan()
		if err != nil {
			return err
		}
		owner := rollupValueString(values[0])
		if p.addDetailRow(owner, owners[owner], values[1:], byKey, res) {
			res.DetailRows++
		}
	}
	return rows.Err()
}

// addDetailRow adds one detail row (group values then sum values), counted times times, and
// reports whether it was used: a row whose group values are all empty is not.
func (p *rollupPlan) addDetailRow(owner string, times int, values []any, byKey map[string]*rollupGroup, res *RollupResult) bool {
	groupValues := make([]string, len(p.groupColumns))
	for i := range p.groupColumns {
		groupValues[i] = rollupValueString(values[i])
	}
	if times == 0 || !slices.ContainsFunc(groupValues, func(v string) bool { return v != "" }) {
		return false
	}
	g := p.groupFor(byKey, groupValues)
	if !g.seen[owner] {
		g.seen[owner] = true
		g.sources += times
	}
	p.addSums(g, values[len(p.groupColumns):], times, res)
	return true
}

// addSums adds a detail row's sum values to its group, reporting values that are not numbers.
func (p *rollupPlan) addSums(g *rollupGroup, values []any, times int, res *RollupResult) {
	for i, col := range p.sumColumns {
		if raw := rollupValueString(values[i]); !g.add(i, raw, times) {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %s %q is not a number", strings.Join(g.values, " "), col, raw))
		}
	}
}

// groupFor returns the group for groupValues, creating it when new.
func (p *rollupPlan) groupFor(byKey map[string]*rollupGroup, groupValues []string) *rollupGroup {
	key := strings.Join(groupValues, "\x00")
	g := byKey[key]
	if g == nil {
		g = &rollupGroup{
			values: groupValues,
			sums:   make([]float64, len(p.sumColumns)),
			hasSum: make([]bool, len(p.sumColumns)),
			seen:   map[string]bool{},
		}
		byKey[key] = g
	}
	return g
}

// add adds raw, times times, to sum i, and reports false when raw is not a number (it is left
// out). An empty value adds nothing.
func (g *rollupGroup) add(i int, raw string, times int) bool {
	if raw == "" {
		return true
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return false
	}
	g.sums[i] += n * float64(times)
	g.hasSum[i] = true
	return true
}

// replaceTargetRows deletes every target row and inserts one row per group.
func (r *Repository) replaceTargetRows(ctx context.Context, tx *sqlx.Tx, p *rollupPlan, groups []*rollupGroup) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+p.target.ref); err != nil {
		return err
	}
	for _, g := range groups {
		insertColumns, placeholders, args := itemInsertColumns(p.target.columns, p.targetValues(g), r.sqlCurrentTimestampExpr())
		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", p.target.ref, strings.Join(insertColumns, ", "), strings.Join(placeholders, ", "))
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}

// targetValues is the target row for a group: empty group values and sums with no numbers stay NULL.
func (p *rollupPlan) targetValues(g *rollupGroup) map[string]string {
	values := map[string]string{}
	for i, col := range p.groupColumns {
		if g.values[i] != "" {
			values[p.targetOf[col]] = g.values[i]
		}
	}
	p.setSumValues(values, g)
	if p.countColumn != "" {
		values[p.countColumn] = strconv.Itoa(g.sources)
	}
	return values
}

func (p *rollupPlan) setSumValues(values map[string]string, g *rollupGroup) {
	for i, col := range p.sumColumns {
		if g.hasSum[i] {
			values[p.targetOf[col]] = formatRollupNumber(g.sums[i])
		}
	}
}

// formatRollupNumber writes a sum without float noise (0.1+0.2 is 0.3) or a trailing ".0".
func formatRollupNumber(n float64) string {
	return strconv.FormatFloat(math.Round(n*1e6)/1e6, 'f', -1, 64)
}

// rollupValueString returns a scanned value as trimmed text; NULL is "".
func rollupValueString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []byte:
		return strings.TrimSpace(string(t))
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// inRollupRange reports whether a scanned date value lies in [from, to), comparing wall-clock times;
// a zero bound is open and an unreadable date is outside every bounded range.
func inRollupRange(v any, from, to time.Time) bool {
	if from.IsZero() && to.IsZero() {
		return true
	}
	t, ok := parseRollupTime(v)
	return ok && notBefore(t, from) && before(t, to)
}

// notBefore reports t >= bound in wall-clock time; a zero bound is open.
func notBefore(t, bound time.Time) bool {
	return bound.IsZero() || !t.Before(wallClock(bound))
}

// before reports t < bound in wall-clock time; a zero bound is open.
func before(t, bound time.Time) bool {
	return bound.IsZero() || t.Before(wallClock(bound))
}

func parseRollupTime(v any) (time.Time, bool) {
	if t, ok := v.(time.Time); ok {
		return wallClock(t), true
	}
	s := rollupValueString(v)
	if t, ok := parseFlexibleDateTime(s); ok {
		return wallClock(t), true
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// wallClock drops t's zone, keeping its clock reading, so stored wall-clock times compare as written.
func wallClock(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

func quoteColumns(cols []string) string {
	return "`" + strings.Join(cols, "`, `") + "`"
}

// equalityWhere builds " WHERE a = ? AND b = ?" (columns already checked), in a stable order.
func equalityWhere(where map[string]string) (string, []any) {
	if len(where) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(where))
	args := make([]any, 0, len(where))
	for _, col := range slices.Sorted(maps.Keys(where)) {
		parts = append(parts, fmt.Sprintf("`%s` = ?", col))
		args = append(args, where[col])
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

// IsSystemTableConfiguration reports whether name is one of SickRock's own system table configurations.
func IsSystemTableConfiguration(name string) bool {
	return slices.ContainsFunc(systemTableConfigs, func(c systemTableConfig) bool { return strings.EqualFold(c.name, name) })
}
