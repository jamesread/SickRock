package repo

import (
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
)

func newInventoryTestRepo(t *testing.T) (*Repository, *sqlx.DB) {
	t.Helper()
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(t.Context(), `
		CREATE TABLE table_configurations (
			name TEXT PRIMARY KEY, db TEXT, "table" TEXT, title TEXT, ordinal INTEGER DEFAULT 0,
			create_button_text TEXT, row_name TEXT, row_name_plural TEXT, create_delegate TEXT, icon TEXT,
			primary_key_column TEXT, default_sort_column TEXT, row_actions TEXT, bulk_entry_json TEXT
		);
		INSERT INTO table_configurations (name, db, "table") VALUES ('inventory', 'main', 'inventory');
		CREATE TABLE inventory (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sr_created TEXT, sr_updated TEXT,
			barcode TEXT, title TEXT, quantity INTEGER, expirydate TEXT
		);
	`); err != nil {
		t.Fatalf("setup schema: %v", err)
	}
	return NewRepository(db), db
}

func TestAddItemQuantity(t *testing.T) {
	r, db := newInventoryTestRepo(t)
	ctx := t.Context()
	beans := map[string]string{"barcode": "123", "expirydate": "2027-03-01 00:00:00"}

	first, err := r.AddItemQuantity(ctx, "inventory", beans, "quantity", 2, map[string]string{"title": "Beans"})
	if err != nil || !first.Created || first.Quantity != 2 {
		t.Fatalf("first add = %+v, %v; want created with quantity 2", first, err)
	}
	if first.Item.Fields["title"] != "Beans" {
		t.Errorf("created title = %v, want Beans", first.Item.Fields["title"])
	}

	second, err := r.AddItemQuantity(ctx, "inventory", beans, "quantity", 3, map[string]string{"title": "ignored"})
	if err != nil || second.Created || second.Quantity != 5 || second.Item.ID != first.Item.ID {
		t.Fatalf("second add = %+v, %v; want same row with quantity 5", second, err)
	}

	otherBatch := map[string]string{"barcode": "123", "expirydate": "2027-06-01 00:00:00"}
	third, err := r.AddItemQuantity(ctx, "inventory", otherBatch, "quantity", 1, nil)
	if err != nil || !third.Created || third.Item.ID == first.Item.ID {
		t.Fatalf("different expiry = %+v, %v; want a new row", third, err)
	}

	noExpiry := map[string]string{"barcode": "123", "expirydate": ""}
	for range 2 {
		if _, err = r.AddItemQuantity(ctx, "inventory", noExpiry, "quantity", 1, nil); err != nil {
			t.Fatalf("no-expiry add: %v", err)
		}
	}

	var rows int
	if err := db.Get(&rows, "SELECT COUNT(*) FROM inventory"); err != nil || rows != 3 {
		t.Fatalf("rows = %d, %v; want 3", rows, err)
	}
	var nullQty int
	if err := db.Get(&nullQty, "SELECT quantity FROM inventory WHERE expirydate IS NULL"); err != nil || nullQty != 2 {
		t.Fatalf("NULL-expiry quantity = %d, %v; want 2", nullQty, err)
	}

	undo, err := r.AddItemQuantity(ctx, "inventory", beans, "quantity", -3, nil)
	if err != nil || undo.Quantity != 2 {
		t.Fatalf("undo = %+v, %v; want quantity 2", undo, err)
	}
	if _, err = r.AddItemQuantity(ctx, "inventory", beans, "quantity", -5, nil); !errors.Is(err, ErrNegativeQuantity) {
		t.Errorf("below zero err = %v, want ErrNegativeQuantity", err)
	}
	if _, err = r.AddItemQuantity(ctx, "inventory", map[string]string{"barcode": "999"}, "quantity", -1, nil); !errors.Is(err, ErrNoMatchingItem) {
		t.Errorf("unmatched negative err = %v, want ErrNoMatchingItem", err)
	}
}

func TestAddItemQuantityRejectsUnknownColumns(t *testing.T) {
	r, _ := newInventoryTestRepo(t)
	ctx := t.Context()
	if _, err := r.AddItemQuantity(ctx, "inventory", map[string]string{"nope": "1"}, "quantity", 1, nil); err == nil {
		t.Error("unknown match column: want error")
	}
	if _, err := r.AddItemQuantity(ctx, "inventory", map[string]string{"barcode": "1"}, "nope", 1, nil); err == nil {
		t.Error("unknown quantity column: want error")
	}
	if _, err := r.AddItemQuantity(ctx, "inventory", nil, "quantity", 1, nil); err == nil {
		t.Error("no match columns: want error")
	}
}
