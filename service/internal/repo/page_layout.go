package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const PageKindItemView = "item_view"

const (
	PageBlockTypeRelation     = "relation"
	PageBlockTypeRelationTabs = "relation_tabs"
	PageBlockTypeQuickJump    = "quick_jump"
)

// PageLayoutBlock is one ordered content block on a page layout.
type PageLayoutBlock struct {
	ID         int    `db:"id"`
	Ordinal    int    `db:"ordinal"`
	BlockType  string `db:"block_type"`
	ConfigJSON string `db:"config_json"`
}

// PageLayoutBlockInput is used when replacing layout blocks.
type PageLayoutBlockInput struct {
	Ordinal    int
	BlockType  string
	ConfigJSON string
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
		var cfg relationBlockConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return nil, fmt.Errorf("invalid relation block config: %w", err)
		}
		name := strings.TrimSpace(cfg.ConstraintName)
		if name == "" {
			return nil, fmt.Errorf("relation block requires constraint_name")
		}
		return []string{name}, nil
	case PageBlockTypeRelationTabs:
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
	case PageBlockTypeQuickJump:
		var cfg quickJumpBlockConfig
		if configJSON != "" && configJSON != "{}" {
			if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
				return nil, fmt.Errorf("invalid quick_jump block config: %w", err)
			}
		}
		return []string{}, nil
	default:
		return nil, fmt.Errorf("unsupported block_type %q", blockType)
	}
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
			if quickJumpCount > 1 {
				return fmt.Errorf("only one quick_jump block is allowed per layout")
			}
			if _, err := constraintNamesFromBlock(blockType, b.ConfigJSON); err != nil {
				return fmt.Errorf("block %d: %w", i+1, err)
			}
			continue
		}
		names, err := constraintNamesFromBlock(blockType, b.ConfigJSON)
		if err != nil {
			return fmt.Errorf("block %d: %w", i+1, err)
		}
		for _, name := range names {
			if _, ok := allowedConstraints[name]; !ok {
				return fmt.Errorf("block %d: unknown constraint %q", i+1, name)
			}
			if _, dup := seen[name]; dup {
				return fmt.Errorf("constraint %q appears in more than one block", name)
			}
			seen[name] = struct{}{}
		}
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
	var layoutID int
	err := r.db.GetContext(ctx, &layoutID, `
		SELECT id FROM page_layouts
		WHERE table_configuration = ? AND page_kind = ?`,
		tableConfiguration, PageKindItemView)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []PageLayoutBlock{}, nil
		}
		return nil, err
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
	if err := validatePageLayoutBlocks(blocks, allowed); err != nil {
		return err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var layoutID int
	err = tx.GetContext(ctx, &layoutID, `
		SELECT id FROM page_layouts
		WHERE table_configuration = ? AND page_kind = ?`,
		tableConfiguration, PageKindItemView)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if len(blocks) == 0 {
				return nil
			}
			res, insErr := tx.ExecContext(ctx, `
				INSERT INTO page_layouts (table_configuration, page_kind) VALUES (?, ?)`,
				tableConfiguration, PageKindItemView)
			if insErr != nil {
				return insErr
			}
			id, insErr := res.LastInsertId()
			if insErr != nil {
				return insErr
			}
			layoutID = int(id)
		} else {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM page_layout_blocks WHERE page_layout_id = ?`, layoutID); err != nil {
		return err
	}

	if len(blocks) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM page_layouts WHERE id = ?`, layoutID); err != nil {
			return err
		}
		return tx.Commit()
	}

	for i, b := range blocks {
		ord := b.Ordinal
		if ord <= 0 {
			ord = i + 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO page_layout_blocks (page_layout_id, ordinal, block_type, config_json)
			VALUES (?, ?, ?, ?)`,
			layoutID, ord, strings.TrimSpace(b.BlockType), strings.TrimSpace(b.ConfigJSON),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}
