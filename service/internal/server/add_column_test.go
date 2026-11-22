package server

import (
	"context"
	"testing"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

func TestAddBarcodeColumnForBulkEntry(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `
		CREATE TABLE pantry (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		INSERT INTO table_configurations (name, title, db, "table") VALUES ('pantry', 'Pantry', 'main', 'pantry');`)
	require.NoError(t, err)
	srv := NewSickRockServer(repo.NewRepository(db), nil)
	ctx := authn.SetInfo(context.Background(), authUserWith(true))

	res, err := srv.AddTableColumn(ctx, connect.NewRequest(&sickrockpb.AddTableColumnRequest{
		PageId:          "pantry",
		Field:           &sickrockpb.Field{Name: "barcode", Type: "barcode"},
		UseForBulkEntry: true,
	}))
	require.NoError(t, err)

	var barcodeType string
	for _, f := range res.Msg.GetFields() {
		if f.GetName() == "barcode" {
			barcodeType = f.GetType()
		}
	}
	assert.Equal(t, "barcode", barcodeType, "the column carries the barcode semantic")
	require.NotNil(t, res.Msg.GetBulkEntry())
	assert.Equal(t, "barcode", res.Msg.GetBulkEntry().GetScanColumn())

	// Without the flag, bulk entry is left alone.
	res, err = srv.AddTableColumn(ctx, connect.NewRequest(&sickrockpb.AddTableColumnRequest{
		PageId: "pantry",
		Field:  &sickrockpb.Field{Name: "ean", Type: "barcode"},
	}))
	require.NoError(t, err)
	assert.Equal(t, "barcode", res.Msg.GetBulkEntry().GetScanColumn())
}
