package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

const PageKindItemView = "item_view"

const (
	PageBlockTypeRelation     = "relation"
	PageBlockTypeRelationTabs = "relation_tabs"
	PageBlockTypeQuickJump    = "quick_jump"
)

// PageLayoutBlock is one ordered content block on a page layout.
type PageLayoutBlock struct {
	BlockType  string `db:"block_type"`
	ConfigJSON string `db:"config_json"`
	ID         int    `db:"id"`
	Ordinal    int    `db:"ordinal"`
}

// PageLayoutBlockInput is used when replacing layout blocks.
type PageLayoutBlockInput struct {
	BlockType  string
	ConfigJSON string
	Ordinal    int
}

type relationBlockConfig struct {
	ConstraintName string `json:"constraint_name"`
}

type relationTabEntry struct {
	ConstraintName string `json:"constraint_name"`
	Label          string `json:"label"`
}

type relationTabsBlockConfig struct {
	SectionTitle string             `json:"section_title"`
	Tabs         []relationTabEntry `json:"tabs"`
}

type quickJumpBlockConfig struct {
	SectionTitle string `json:"section_title"`
}

func constraintNamesFromBlock(blockType, configJSON string) ([]string, error) {
	blockType = strings.TrimSpace(blockType)
	configJSON = strings.TrimSpace(configJSON)
	switch blockType {
	case PageBlockTypeRelation:
		return relationBlockConstraintNames(configJSON)
	case PageBlockTypeRelationTabs:
		return relationTabsConstraintNames(configJSON)
	case PageBlockTypeQuickJump:
		return quickJumpConstraintNames(configJSON)
	default:
		return nil, fmt.Errorf("unsupported block_type %q", blockType)
	}
}

func relationBlockConstraintNames(configJSON string) ([]string, error) {
	var cfg relationBlockConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("invalid relation block config: %w", err)
	}
	name := strings.TrimSpace(cfg.ConstraintName)
	if name == "" {
		return nil, fmt.Errorf("relation block requires constraint_name")
	}
	return []string{name}, nil
}

func relationTabsConstraintNames(configJSON string) ([]string, error) {
	var cfg relationTabsBlockConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("invalid relation_tabs block config: %w", err)
	}
	if len(cfg.Tabs) == 0 {
		return nil, fmt.Errorf("relation_tabs block requires at least one tab")
	}
	out := make([]string, 0, len(cfg.Tabs))
	for i, tab := range cfg.Tabs {
		name := strings.TrimSpace(tab.ConstraintName)
		if name == "" {
			return nil, fmt.Errorf("tab %d requires constraint_name", i+1)
		}
		out = append(out, name)
	}
	return out, nil
}

// quickJumpConstraintNames validates a quick_jump config (which may be empty); the block references no relations.
func quickJumpConstraintNames(configJSON string) ([]string, error) {
	if configJSON == "" || configJSON == "{}" {
		return []string{}, nil
	}
	var cfg quickJumpBlockConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("invalid quick_jump block config: %w", err)
	}
	return []string{}, nil
}

func validatePageLayoutBlocks(blocks []PageLayoutBlockInput, allowedConstraints map[string]struct{}) error {
	seen := make(map[string]struct{})
	quickJumpCount := 0
	for i, b := range blocks {
		blockType := strings.TrimSpace(b.BlockType)
		if blockType == "" {
			return fmt.Errorf("block %d: block_type is required", i+1)
		}
		if blockType == PageBlockTypeQuickJump {
			quickJumpCount++
		}
		if quickJumpCount > 1 {
			return fmt.Errorf("only one quick_jump block is allowed per layout")
		}
		names, err := constraintNamesFromBlock(blockType, b.ConfigJSON)
		if err != nil {
			return fmt.Errorf("block %d: %w", i+1, err)
		}
		if err := claimLayoutConstraints(i, names, allowedConstraints, seen); err != nil {
			return err
		}
	}
	return nil
}

// claimLayoutConstraints checks block i's relation constraints are allowed and not used by an earlier block.
func claimLayoutConstraints(i int, names []string, allowedConstraints, seen map[string]struct{}) error {
	for _, name := range names {
		if _, ok := allowedConstraints[name]; !ok {
			return fmt.Errorf("block %d: unknown constraint %q", i+1, name)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("constraint %q appears in more than one block", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func (r *Repository) allowedRelationConstraintNames(ctx context.Context, tableConfiguration string) (map[string]struct{}, error) {
	rels, err := r.listTableRelationsForConfiguration(ctx, tableConfiguration)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(rels))
	for _, rel := range rels {
		out[rel.ConstraintName] = struct{}{}
	}
	return out, nil
}

// GetItemPageLayoutBlocks returns layout blocks for item view, or empty slice if none.
func (r *Repository) GetItemPageLayoutBlocks(ctx context.Context, tableConfiguration string) ([]PageLayoutBlock, error) {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	if tableConfiguration == "" {
		return nil, fmt.Errorf("table configuration is required")
	}
	return r.loadItemPageLayoutBlocks(ctx, tableConfiguration)
}

func (r *Repository) itemPageLayoutID(ctx context.Context, tableConfiguration string) (int, error) {
	var layoutID int
	err := r.db.GetContext(ctx, &layoutID, `
		SELECT id FROM page_layouts
		WHERE table_configuration = ? AND page_kind = ?`,
		tableConfiguration, PageKindItemView)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return layoutID, err
}

func (r *Repository) loadItemPageLayoutBlocks(ctx context.Context, tableConfiguration string) ([]PageLayoutBlock, error) {
	layoutID, err := r.itemPageLayoutID(ctx, tableConfiguration)
	if err != nil {
		return nil, err
	}
	if layoutID == 0 {
		return []PageLayoutBlock{}, nil
	}
	var blocks []PageLayoutBlock
	err = r.db.SelectContext(ctx, &blocks, `
		SELECT id, ordinal, block_type, config_json
		FROM page_layout_blocks
		WHERE page_layout_id = ?
		ORDER BY ordinal ASC, id ASC`, layoutID)
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

// SetItemPageLayoutBlocks replaces all item view layout blocks for a table configuration.
func (r *Repository) SetItemPageLayoutBlocks(ctx context.Context, tableConfiguration string, blocks []PageLayoutBlockInput) error {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	if tableConfiguration == "" {
		return fmt.Errorf("table configuration is required")
	}
	allowed, err := r.allowedRelationConstraintNames(ctx, tableConfiguration)
	if err != nil {
		return err
	}
	if err = validatePageLayoutBlocks(blocks, allowed); err != nil {
		return err
	}

	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		layoutID, err := ensureItemPageLayout(ctx, tx, tableConfiguration, len(blocks) > 0)
		if err != nil || layoutID == 0 {
			return err
		}
		return writePageLayoutBlocks(ctx, tx, layoutID, blocks)
	})
}

// ensureItemPageLayout returns the id of the table's item-view page layout, creating it when it is
// missing and create is set. It returns 0 when there is no layout and none was created.
func ensureItemPageLayout(ctx context.Context, tx *sqlx.Tx, tableConfiguration string, create bool) (int, error) {
	layoutID, exists, err := itemPageLayoutID(ctx, tx, tableConfiguration)
	if err != nil || exists || !create {
		return layoutID, err
	}
	return insertItemPageLayout(ctx, tx, tableConfiguration)
}

// itemPageLayoutID returns the id of the table's item-view page layout, and whether it exists.
func itemPageLayoutID(ctx context.Context, tx *sqlx.Tx, tableConfiguration string) (int, bool, error) {
	var layoutID int
	err := tx.GetContext(ctx, &layoutID, `
		SELECT id FROM page_layouts
		WHERE table_configuration = ? AND page_kind = ?`,
		tableConfiguration, PageKindItemView)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return layoutID, err == nil, err
}

func insertItemPageLayout(ctx context.Context, tx *sqlx.Tx, tableConfiguration string) (int, error) {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO page_layouts (table_configuration, page_kind) VALUES (?, ?)`,
		tableConfiguration, PageKindItemView)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

// writePageLayoutBlocks replaces the layout's blocks, or deletes the layout itself when there are none.
func writePageLayoutBlocks(ctx context.Context, tx *sqlx.Tx, layoutID int, blocks []PageLayoutBlockInput) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM page_layout_blocks WHERE page_layout_id = ?`, layoutID); err != nil {
		return err
	}
	if len(blocks) == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM page_layouts WHERE id = ?`, layoutID)
		return err
	}
	for i, b := range blocks {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO page_layout_blocks (page_layout_id, ordinal, block_type, config_json)
			VALUES (?, ?, ?, ?)`,
			layoutID, ordinalOrPosition(b.Ordinal, i), strings.TrimSpace(b.BlockType), strings.TrimSpace(b.ConfigJSON),
		); err != nil {
			return err
		}
	}
	return nil
}
