package server

import "testing"

func TestRowActionsToProto(t *testing.T) {
	got := rowActionsToProto("table_apps", `[
		{"label":"Create table in this app","path":"/admin/table/create","query":{"navigationAppId":"{id}"}},
		{"label":"","path":"/skipped"},
		{"label":"No path"}
	]`)
	if len(got) != 1 {
		t.Fatalf("got %d actions, want 1", len(got))
	}
	if got[0].GetPath() != "/admin/table/create" || got[0].GetQuery()["navigationAppId"] != "{id}" {
		t.Fatalf("unexpected action: %+v", got[0])
	}
}

func TestRowActionsToProtoEmptyOrInvalid(t *testing.T) {
	for _, raw := range []string{"", "   ", "not json", `{"label":"x"}`} {
		if got := rowActionsToProto("t", raw); len(got) != 0 {
			t.Errorf("rowActionsToProto(%q) = %d actions, want 0", raw, len(got))
		}
	}
}
