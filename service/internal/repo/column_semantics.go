package repo

import (
	"context"
	"fmt"
	"strings"
)

const (
	SemanticTypeUserRef  = "user_ref"
	SemanticTypeDatetime = "datetime"
)

func IsDatetimeTypeName(typeName string) bool {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "datetime", "date", "time", "timestamp":
		return true
	default:
		return false
	}
}

func (r *Repository) SetColumnSemantic(ctx context.Context, tableConfiguration, columnName, semanticType string) error {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	columnName = sanitizeDatabaseIdentifier(columnName)
	semanticType = strings.TrimSpace(semanticType)
	if tableConfiguration == "" || columnName == "" || semanticType == "" {
		return fmt.Errorf("table configuration, column name, and semantic type are required")
	}
	const q = `
		INSERT INTO table_column_semantics (table_configuration, column_name, semantic_type)
		VALUES (?, ?, ?)
		ON CONFLICT(table_configuration, column_name) DO UPDATE SET semantic_type = excluded.semantic_type`
	switch r.db.DriverName() {
	case "mysql":
		const mysqlQ = `
			INSERT INTO table_column_semantics (table_configuration, column_name, semantic_type)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE semantic_type = VALUES(semantic_type)`
		_, err := r.db.ExecContext(ctx, mysqlQ, tableConfiguration, columnName, semanticType)
		return err
	default:
		_, err := r.db.ExecContext(ctx, q, tableConfiguration, columnName, semanticType)
		return err
	}
}

func (r *Repository) GetColumnSemantics(ctx context.Context, tableConfiguration string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(tableConfiguration) == "" {
		return out, nil
	}
	rows, err := r.db.QueryxContext(ctx, `
		SELECT column_name, semantic_type
		FROM table_column_semantics
		WHERE table_configuration = ?`, tableConfiguration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var columnName, semanticType string
		if err := rows.Scan(&columnName, &semanticType); err != nil {
			return nil, err
		}
		out[columnName] = semanticType
	}
	return out, rows.Err()
}

func (r *Repository) ApplyColumnSemantics(ctx context.Context, tableConfiguration string, specs []FieldSpec) []FieldSpec {
	semantics, err := r.GetColumnSemantics(ctx, tableConfiguration)
	if err != nil || len(semantics) == 0 {
		return specs
	}
	for i, spec := range specs {
		if semantic, ok := semantics[spec.Name]; ok && semantic != "" {
			specs[i].Type = semantic
		}
	}
	return specs
}

func (r *Repository) DeleteColumnSemantic(ctx context.Context, tableConfiguration, columnName string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM table_column_semantics WHERE table_configuration = ? AND column_name = ?`,
		tableConfiguration, sanitizeDatabaseIdentifier(columnName))
	return err
}
