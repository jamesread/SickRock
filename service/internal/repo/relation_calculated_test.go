package repo

import (
	"testing"
)

func TestNormalizeRelationExpr(t *testing.T) {
	got := normalizeRelationExpr("{skus.value} * {quote_lines.quantity}")
	want := "skus.value * quote_lines.quantity"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRelationCalcStorageKey(t *testing.T) {
	if RelationCalcStorageKey("Line total", 2) != "__calc_line_total" {
		t.Fatalf("unexpected slug key")
	}
	if RelationCalcStorageKey("  ", 1) != "__calc_1" {
		t.Fatalf("expected index fallback")
	}
}

func TestEvaluateRelationCalculatedExpr(t *testing.T) {
	env := map[string]any{
		"skus": map[string]any{
			"value": float64(10),
		},
		"quote_lines": map[string]any{
			"quantity": float64(3),
		},
	}
	got, err := evaluateRelationCalculatedExpr("{skus.value} * {quote_lines.quantity}", env)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got != "30" {
		t.Fatalf("got %q want 30", got)
	}
}
