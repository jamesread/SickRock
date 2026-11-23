package repo

import (
	"errors"
	"testing"
	"time"

	"github.com/jamesread/SickRock/internal/testdb"
	"github.com/jmoiron/sqlx"
)

// newRollupTestRepo builds a meal planner: plan rows link to recipes, whose ingredient rows are
// rolled up into shopping_list.
func newRollupTestRepo(t *testing.T) (*Repository, *sqlx.DB) {
	t.Helper()
	db := testdb.NewMigratedSQLite(t)
	if _, err := db.ExecContext(t.Context(), `
		INSERT INTO table_configurations (name, title, db, "table") VALUES
			('meal_plan', 'Meal plan', 'main', 'meal_plan'),
			('recipes', 'Recipes', 'main', 'recipes'),
			('recipe_ingredients', 'Recipe ingredients', 'main', 'recipe_ingredients'),
			('shopping_list', 'Shopping list', 'main', 'shopping_list');
		CREATE TABLE recipes (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		CREATE TABLE meal_plan (id INTEGER PRIMARY KEY AUTOINCREMENT, datetime TEXT, recipe INTEGER, cook TEXT);
		CREATE TABLE recipe_ingredients (
			id INTEGER PRIMARY KEY AUTOINCREMENT, recipe INTEGER, ingredient TEXT, unit TEXT, quantity TEXT
		);
		CREATE TABLE shopping_list (
			id INTEGER PRIMARY KEY AUTOINCREMENT, sr_created TEXT, sr_updated TEXT,
			item TEXT, unit TEXT, quantity REAL, meals INTEGER
		);
		INSERT INTO recipes (id, name) VALUES (1, 'Chilli'), (2, 'Soup'), (3, 'Toast');
		INSERT INTO recipe_ingredients (recipe, ingredient, unit, quantity) VALUES
			(1, 'Onion', 'g', '200'), (1, 'Beans', 'tin', '1'), (1, 'Chilli powder', 'tsp', '0.1'),
			(2, 'Onion', 'g', '150'), (2, 'Onion', 'whole', '1'), (2, 'Chilli powder', 'tsp', '0.2'),
			(2, 'Salt', '', 'a pinch'), (2, '', '', '5'),
			(3, 'Bread', 'slice', '2');
		INSERT INTO meal_plan (datetime, recipe, cook) VALUES
			('2026-11-22 18:00:00', 3, 'sam'),  -- the Sunday before the week
			('2026-11-23T08:00', 1, 'sam'),      -- Monday, first moment of the week
			('2026-11-25 18:00', 2, 'alex'),
			('2026-11-27 18:00:00', 1, 'alex'), -- chilli again
			('2026-11-30 00:00:00', 3, 'sam'),  -- next Monday, outside
			('not a date', 3, 'sam');
	`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return NewRepository(db), db
}

func shoppingRollup() *Rollup {
	return &Rollup{
		ID: 1, Name: "Shopping list",
		SourceTable: "meal_plan", SourceLinkColumn: "recipe",
		DetailTable: "recipe_ingredients", DetailLinkColumn: "recipe",
		GroupColumns: []string{"ingredient", "unit"}, SumColumns: []string{"quantity"},
		TargetTable: "shopping_list", ColumnMap: map[string]string{"ingredient": "item"}, CountColumn: "meals",
	}
}

type shoppingRow struct {
	Item     string   `db:"item"`
	Unit     *string  `db:"unit"`
	Quantity *float64 `db:"quantity"`
	Meals    int      `db:"meals"`
}

func shoppingRows(t *testing.T, db *sqlx.DB) map[string]shoppingRow {
	t.Helper()
	var rows []shoppingRow
	if err := db.Select(&rows, `SELECT item, unit, quantity, meals FROM shopping_list`); err != nil {
		t.Fatalf("read shopping list: %v", err)
	}
	out := map[string]shoppingRow{}
	for _, row := range rows {
		unit := ""
		if row.Unit != nil {
			unit = *row.Unit
		}
		out[row.Item+"/"+unit] = row
	}
	return out
}

func week() RollupSourceFilter {
	return RollupSourceFilter{
		DateColumn: "datetime",
		From:       time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
	}
}

func TestRunRollupWeek(t *testing.T) {
	r, db := newRollupTestRepo(t)
	res, err := r.RunRollup(t.Context(), shoppingRollup(), week())
	if err != nil {
		t.Fatalf("RunRollup: %v", err)
	}
	if res.SourceRows != 3 || res.RowsWritten != 5 {
		t.Errorf("result = %+v; want 3 source rows, 5 written", res)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("skipped = %v; want the pinch of salt", res.Skipped)
	}

	rows := shoppingRows(t, db)
	want := map[string]struct {
		qty   float64
		meals int
	}{
		"Onion/g":           {550, 3}, // 200 x 2 chillis + 150
		"Onion/whole":       {1, 1},
		"Beans/tin":         {2, 2},
		"Chilli powder/tsp": {0.4, 3},
	}
	for key, w := range want {
		row, ok := rows[key]
		if !ok || row.Quantity == nil || *row.Quantity != w.qty || row.Meals != w.meals {
			t.Errorf("%s = %+v; want quantity %v from %d meals", key, row, w.qty, w.meals)
		}
	}
	if salt, ok := rows["Salt/"]; !ok || salt.Quantity != nil {
		t.Errorf("salt = %+v; want a row with no quantity", salt)
	}
	if _, ok := rows["Bread/slice"]; ok {
		t.Error("toast from outside the week was included")
	}
}

func TestRunRollupReplacesTarget(t *testing.T) {
	r, db := newRollupTestRepo(t)
	ctx := t.Context()
	if _, err := db.ExecContext(ctx, `INSERT INTO shopping_list (item) VALUES ('Old')`); err != nil {
		t.Fatal(err)
	}
	if err := r.SetTickListCompletion(ctx, "shopping_list", "1", true); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := r.RunRollup(ctx, shoppingRollup(), week()); err != nil {
			t.Fatalf("RunRollup: %v", err)
		}
	}
	rows := shoppingRows(t, db)
	if _, ok := rows["Old/"]; ok || len(rows) != 5 {
		t.Errorf("rows = %v; want the 5 rolled-up rows only", rows)
	}
	if q := rows["Beans/tin"].Quantity; q == nil || *q != 2 {
		t.Errorf("beans after a second run = %v; want 2, not added again", q)
	}
	state, err := r.GetTickListState(ctx, "shopping_list")
	if err != nil || len(state) != 0 {
		t.Errorf("tick state = %v, %v; want cleared", state, err)
	}
}

func TestRunRollupWhereAndOpenRange(t *testing.T) {
	r, db := newRollupTestRepo(t)
	filter := RollupSourceFilter{Where: map[string]string{"cook": "sam"}}
	res, err := r.RunRollup(t.Context(), shoppingRollup(), filter)
	if err != nil {
		t.Fatalf("RunRollup: %v", err)
	}
	if res.SourceRows != 4 {
		t.Errorf("source rows = %d; want sam's 4 meals", res.SourceRows)
	}
	if q := shoppingRows(t, db)["Bread/slice"].Quantity; q == nil || *q != 6 {
		t.Errorf("bread = %v; want 6 slices from 3 toasts", q)
	}
}

func TestRunRollupSourceIDs(t *testing.T) {
	r, db := newRollupTestRepo(t)
	// Rows 2 and 3 are Monday's chilli and Wednesday's soup; 1 is outside the week.
	filter := week()
	filter.LimitToSourceIDs, filter.SourceIDs = true, []string{"1", "2", "3"}
	res, err := r.RunRollup(t.Context(), shoppingRollup(), filter)
	if err != nil {
		t.Fatalf("RunRollup: %v", err)
	}
	if res.SourceRows != 2 {
		t.Errorf("source rows = %d; want 2 (listed and in the week)", res.SourceRows)
	}
	if q := shoppingRows(t, db)["Beans/tin"].Quantity; q == nil || *q != 1 {
		t.Errorf("beans = %v; want 1 tin from one chilli", q)
	}

	filter = RollupSourceFilter{LimitToSourceIDs: true}
	if res, err = r.RunRollup(t.Context(), shoppingRollup(), filter); err != nil || res.SourceRows != 0 || res.RowsWritten != 0 {
		t.Errorf("empty id list = %+v, %v; want no rows", res, err)
	}
}

func TestRunRollupDirectDetails(t *testing.T) {
	r, db := newRollupTestRepo(t)
	ru := shoppingRollup()
	ru.SourceTable, ru.SourceLinkColumn = "recipes", ""
	if _, err := r.RunRollup(t.Context(), ru, RollupSourceFilter{Where: map[string]string{"name": "Soup"}}); err != nil {
		t.Fatalf("RunRollup: %v", err)
	}
	if q := shoppingRows(t, db)["Onion/g"].Quantity; q == nil || *q != 150 {
		t.Errorf("onion = %v; want the soup's 150 g", q)
	}
}

func TestRunRollupRefusesBadConfig(t *testing.T) {
	r, _ := newRollupTestRepo(t)
	cases := map[string]func(*Rollup, *RollupSourceFilter){
		"unknown group column": func(ru *Rollup, _ *RollupSourceFilter) { ru.GroupColumns = []string{"nope"} },
		"no group columns":     func(ru *Rollup, _ *RollupSourceFilter) { ru.GroupColumns = nil },
		"unknown target":       func(ru *Rollup, _ *RollupSourceFilter) { ru.ColumnMap = map[string]string{"ingredient": "nope"} },
		"unknown filter":       func(_ *Rollup, f *RollupSourceFilter) { f.Where = map[string]string{"x`; DROP": "1"} },
		"unknown date column":  func(_ *Rollup, f *RollupSourceFilter) { f.DateColumn = "when" },
		"target is detail":     func(ru *Rollup, _ *RollupSourceFilter) { ru.TargetTable = "recipe_ingredients" },
		"system target":        func(ru *Rollup, _ *RollupSourceFilter) { ru.TargetTable = "table_navigation" },
		"grouped and summed":   func(ru *Rollup, _ *RollupSourceFilter) { ru.SumColumns = []string{"unit"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			ru, filter := shoppingRollup(), week()
			mutate(ru, &filter)
			if _, err := r.RunRollup(t.Context(), ru, filter); err == nil {
				t.Error("RunRollup succeeded; want an error")
			}
		})
	}
}

func TestGetAndListRollups(t *testing.T) {
	r, db := newRollupTestRepo(t)
	ctx := t.Context()
	if _, err := db.ExecContext(ctx, `INSERT INTO rollups (name, source_table, source_link_column, detail_table, detail_link_column,
		group_columns_json, sum_columns_json, target_table, column_map_json)
		VALUES ('Shopping list', 'meal_plan', 'recipe', 'recipe_ingredients', 'recipe', '["ingredient", "unit", ""]', '["quantity"]',
			'shopping_list', '{"ingredient":"item"}')`); err != nil {
		t.Fatal(err)
	}
	list, err := r.ListRollups(ctx, "meal_plan")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListRollups = %v, %v; want one", list, err)
	}
	if other, _ := r.ListRollups(ctx, "recipes"); len(other) != 0 {
		t.Errorf("rollups for recipes = %v; want none", other)
	}
	ru, err := r.GetRollup(ctx, list[0].ID)
	if err != nil || len(ru.GroupColumns) != 2 || ru.ColumnMap["ingredient"] != "item" {
		t.Fatalf("GetRollup = %+v, %v", ru, err)
	}
	if _, err := r.RunRollup(ctx, ru, week()); err != nil {
		t.Errorf("RunRollup of stored rollup: %v", err)
	}
	if _, err := r.GetRollup(ctx, 999); !errors.Is(err, ErrRollupNotFound) {
		t.Errorf("GetRollup(999) error = %v; want ErrRollupNotFound", err)
	}
}
