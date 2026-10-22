package repo

import "testing"

func TestMapFieldsForExprCaseInsensitiveKeys(t *testing.T) {
	out := mapFieldsForExpr(map[string]any{
		"Quantity": float64(4),
	})
	if out["quantity"] != float64(4) {
		t.Fatalf("expected lowercase alias, got %+v", out)
	}
}

func TestEvaluateRelationCalculatedExprEmptySku(t *testing.T) {
	env := map[string]any{
		"skus":        map[string]any{},
		"quote_lines": map[string]any{"quantity": float64(2)},
	}
	_, err := evaluateRelationCalculatedExpr("{skus.value} * {quote_lines.quantity}", env)
	if err == nil {
		t.Fatal("expected error for missing skus.value")
	}
}
