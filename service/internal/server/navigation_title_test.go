package server

import (
	"database/sql"
	"testing"

	repo "github.com/jamesread/SickRock/internal/repo"
)

func TestDisplayNavigationTitle(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{in: "customer_website", want: "Customer Website"},
		{in: "table_settings", want: "Table Settings"},
		{in: "Table Configurations", want: "Table Configurations"},
		{in: "  __already__titled__  ", want: "Already Titled"},
		{in: "", want: ""},
		{in: "___", want: ""},
	}
	for _, tc := range cases {
		got := displayNavigationTitle(tc.in)
		if got != tc.want {
			t.Errorf("displayNavigationTitle(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNavigationLinkTitleOverride(t *testing.T) {
	withOverride := repo.NavigationItem{
		Navigation:    sql.NullString{String: "customer_website", Valid: true},
		TitleOverride: sql.NullString{String: "  Accounts  ", Valid: true},
	}
	if got := navigationLinkTitle(withOverride); got != "Accounts" {
		t.Fatalf("override title = %q", got)
	}

	calculated := repo.NavigationItem{
		Navigation: sql.NullString{String: "customer_website", Valid: true},
	}
	if got := navigationLinkTitle(calculated); got != "Customer Website" {
		t.Fatalf("calculated title = %q", got)
	}

	blankOverride := repo.NavigationItem{
		Navigation:    sql.NullString{String: "open_deals", Valid: true},
		TitleOverride: sql.NullString{String: "   ", Valid: true},
	}
	if got := navigationLinkTitle(blankOverride); got != "Open Deals" {
		t.Fatalf("blank override title = %q", got)
	}
}
