package repo

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jmoiron/sqlx"
)

func setupPageLayoutTestDB(t *testing.T) *Repository {
	t.Helper()
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE page_layouts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			table_configuration TEXT NOT NULL,
			page_kind TEXT NOT NULL DEFAULT 'item_view',
			UNIQUE (table_configuration, page_kind)
		);
		CREATE TABLE page_layout_blocks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			page_layout_id INTEGER NOT NULL,
			ordinal INTEGER NOT NULL DEFAULT 0,
			block_type TEXT NOT NULL,
			config_json TEXT NOT NULL,
			FOREIGN KEY (page_layout_id) REFERENCES page_layouts(id) ON DELETE CASCADE
		);
		CREATE TABLE table_relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			constraint_name TEXT NOT NULL UNIQUE,
			source_table_key TEXT NOT NULL,
			source_column TEXT NOT NULL,
			referenced_table_key TEXT NOT NULL,
			referenced_column TEXT NOT NULL,
			on_delete_action TEXT NOT NULL DEFAULT 'NO ACTION',
			on_update_action TEXT NOT NULL DEFAULT 'NO ACTION',
			display_name TEXT,
			pull_through_json TEXT,
			absorb_into_item INTEGER NOT NULL DEFAULT 0,
			absorb_type TEXT
		);
		INSERT INTO table_relations (
			constraint_name, source_table_key, source_column,
			referenced_table_key, referenced_column
		) VALUES
			('fk_a', 'orders', 'customer_id', 'customers', 'id'),
			('fk_b', 'orders', 'ship_id', 'ships', 'id');
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return NewRepository(db)
}

func TestSetItemPageLayoutBlocksSaveLoadAndEmpty(t *testing.T) {
	repo := setupPageLayoutTestDB(t)
	ctx := context.Background()
	cfg, _ := json.Marshal(relationBlockConfig{ConstraintName: "fk_a"})
	if err := repo.SetItemPageLayoutBlocks(ctx, "orders", []PageLayoutBlockInput{
		{Ordinal: 1, BlockType: PageBlockTypeRelation, ConfigJSON: string(cfg)},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	blocks, err := repo.GetItemPageLayoutBlocks(ctx, "orders")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(blocks) != 1 || blocks[0].BlockType != PageBlockTypeRelation {
		t.Fatalf("blocks: %+v", blocks)
	}
	if err := repo.SetItemPageLayoutBlocks(ctx, "orders", nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	blocks, err = repo.GetItemPageLayoutBlocks(ctx, "orders")
	if err != nil {
		t.Fatalf("load after clear: %v", err)
	}
	if len(blocks) != 0 {
		t.Fatalf("expected empty layout, got %+v", blocks)
	}
}

func TestSetItemPageLayoutBlocksValidation(t *testing.T) {
	repo := setupPageLayoutTestDB(t)
	ctx := context.Background()
	cfg, _ := json.Marshal(relationBlockConfig{ConstraintName: "fk_missing"})
	err := repo.SetItemPageLayoutBlocks(ctx, "orders", []PageLayoutBlockInput{
		{Ordinal: 1, BlockType: PageBlockTypeRelation, ConfigJSON: string(cfg)},
	})
	if err == nil {
		t.Fatal("expected unknown constraint error")
	}

	cfgA, _ := json.Marshal(relationBlockConfig{ConstraintName: "fk_a"})
	cfgB, _ := json.Marshal(relationBlockConfig{ConstraintName: "fk_a"})
	err = repo.SetItemPageLayoutBlocks(ctx, "orders", []PageLayoutBlockInput{
		{Ordinal: 1, BlockType: PageBlockTypeRelation, ConfigJSON: string(cfgA)},
		{Ordinal: 2, BlockType: PageBlockTypeRelation, ConfigJSON: string(cfgB)},
	})
	if err == nil {
		t.Fatal("expected duplicate constraint error")
	}

	tabCfg, _ := json.Marshal(relationTabsBlockConfig{
		Tabs: []relationTabEntry{
			{ConstraintName: "fk_a"},
			{ConstraintName: "fk_b"},
		},
	})
	if err := repo.SetItemPageLayoutBlocks(ctx, "orders", []PageLayoutBlockInput{
		{Ordinal: 1, BlockType: PageBlockTypeRelationTabs, ConfigJSON: string(tabCfg)},
	}); err != nil {
		t.Fatalf("tabs save: %v", err)
	}
}

func TestSetItemPageLayoutBlocksQuickJump(t *testing.T) {
	repo := setupPageLayoutTestDB(t)
	ctx := context.Background()
	jumpCfg, _ := json.Marshal(quickJumpBlockConfig{SectionTitle: "On this page"})
	relCfg, _ := json.Marshal(relationBlockConfig{ConstraintName: "fk_a"})
	if err := repo.SetItemPageLayoutBlocks(ctx, "orders", []PageLayoutBlockInput{
		{Ordinal: 1, BlockType: PageBlockTypeQuickJump, ConfigJSON: string(jumpCfg)},
		{Ordinal: 2, BlockType: PageBlockTypeRelation, ConfigJSON: string(relCfg)},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	err := repo.SetItemPageLayoutBlocks(ctx, "orders", []PageLayoutBlockInput{
		{Ordinal: 1, BlockType: PageBlockTypeQuickJump, ConfigJSON: `{}`},
		{Ordinal: 2, BlockType: PageBlockTypeQuickJump, ConfigJSON: `{}`},
	})
	if err == nil {
		t.Fatal("expected duplicate quick_jump error")
	}
}
