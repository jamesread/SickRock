package repo

import (
	"testing"

	"github.com/jamesread/SickRock/internal/testdb"
)

func TestNormalizeRelationQuickAddJSON(t *testing.T) {
	got, err := NormalizeRelationQuickAddJSON(`{"enabled":true,"columns":[" ingredient","quantity","","quantity"],"createMissing":true,"extra":1}`)
	if err != nil || got != `{"columns":["ingredient","quantity"],"enabled":true,"createMissing":true}` {
		t.Errorf("normalized = %s, %v", got, err)
	}
	if got, err = NormalizeRelationQuickAddJSON("  "); err != nil || got != "" {
		t.Errorf("blank = %q, %v; want cleared", got, err)
	}
	if _, err = NormalizeRelationQuickAddJSON(`{"columns":"x"}`); err == nil {
		t.Error("bad columns accepted")
	}
}

func TestSetTableRelationQuickAdd(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	ctx := t.Context()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO table_configurations (name, title, db, "table") VALUES
			('recipes', 'Recipes', 'main', 'recipes'), ('recipe_ingredients', 'Recipe ingredients', 'main', 'recipe_ingredients');
		CREATE TABLE recipes (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		CREATE TABLE recipe_ingredients (id INTEGER PRIMARY KEY AUTOINCREMENT, recipe INTEGER, quantity TEXT);`); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db)
	if err := r.CreateForeignKey(ctx, "recipe_ingredients", "recipe", "recipes", "id", "CASCADE", "CASCADE"); err != nil {
		t.Fatalf("CreateForeignKey: %v", err)
	}
	fks, err := r.GetForeignKeys(ctx, "recipe_ingredients")
	if err != nil || len(fks) != 1 {
		t.Fatalf("GetForeignKeys = %v, %v", fks, err)
	}
	name := fks[0].ConstraintName
	if err = r.SetTableRelationQuickAdd(ctx, name, `{"enabled":true,"columns":["quantity"]}`); err != nil {
		t.Fatalf("SetTableRelationQuickAdd: %v", err)
	}
	fks, _ = r.GetForeignKeys(ctx, "recipe_ingredients")
	if fks[0].QuickAddJSON != `{"columns":["quantity"],"enabled":true,"createMissing":false}` {
		t.Errorf("stored = %s", fks[0].QuickAddJSON)
	}
	if err = r.SetTableRelationQuickAdd(ctx, name, ""); err != nil {
		t.Fatal(err)
	}
	if fks, _ = r.GetForeignKeys(ctx, "recipe_ingredients"); fks[0].QuickAddJSON != "" {
		t.Errorf("after clearing = %s", fks[0].QuickAddJSON)
	}
	if err = r.SetTableRelationQuickAdd(ctx, "nope", "{}"); err == nil {
		t.Error("unknown relation accepted")
	}
}
