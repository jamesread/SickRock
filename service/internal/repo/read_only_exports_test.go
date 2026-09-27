package repo

import (
	"testing"
	"time"
)

func TestParseAllowedGroupIDs(t *testing.T) {
	ids, err := parseAllowedGroupIDs(`[1,2,3]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[2] != 3 {
		t.Fatalf("unexpected ids: %v", ids)
	}
	empty, err := parseAllowedGroupIDs("[]")
	if err != nil || len(empty) != 0 {
		t.Fatalf("expected empty slice, got %v err=%v", empty, err)
	}
}

func TestFilterItemsWeekendsOnly(t *testing.T) {
	sat := Item{Fields: map[string]interface{}{"calendar_date": "2026-09-26"}}
	sun := Item{Fields: map[string]interface{}{"calendar_date": "2026-09-27"}}
	mon := Item{Fields: map[string]interface{}{"calendar_date": "2026-09-28"}}
	out := filterItemsWeekendsOnly([]Item{sat, sun, mon})
	if len(out) != 2 {
		t.Fatalf("expected 2 weekend items, got %d", len(out))
	}
}

func TestParseFlexibleDateTime(t *testing.T) {
	dt, ok := parseFlexibleDateTime("2026-09-27 10:00:00")
	if !ok || dt.Weekday() != time.Sunday {
		t.Fatalf("unexpected parse: %v ok=%v", dt, ok)
	}
}
