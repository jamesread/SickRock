package server

import "testing"

func TestBulkEntryToProto(t *testing.T) {
	cols := []string{"id", "barcode", "title", "quantity", "expirydate"}
	got := bulkEntryToProto("inventory", `{"scan_column":"barcode","quantity_column":"quantity",
		"match_columns":["expirydate","nope","barcode"],"fields":["title","quantity","missing"],"save_on_scan":true}`, cols)
	if got == nil {
		t.Fatal("want config")
	}
	if got.GetScanColumn() != "barcode" || got.GetQuantityColumn() != "quantity" || !got.GetSaveOnScan() {
		t.Errorf("unexpected config: %+v", got)
	}
	if len(got.GetMatchColumns()) != 1 || got.GetMatchColumns()[0] != "expirydate" {
		t.Errorf("match columns = %v, want [expirydate]", got.GetMatchColumns())
	}
	if len(got.GetFields()) != 1 || got.GetFields()[0] != "title" {
		t.Errorf("fields = %v, want [title]", got.GetFields())
	}
}

func TestBulkEntryToProtoOff(t *testing.T) {
	cols := []string{"barcode"}
	for _, raw := range []string{"", "not json", `{"scan_column":"missing"}`, `{}`} {
		if got := bulkEntryToProto("t", raw, cols); got != nil {
			t.Errorf("bulkEntryToProto(%q) = %+v, want nil", raw, got)
		}
	}
}
