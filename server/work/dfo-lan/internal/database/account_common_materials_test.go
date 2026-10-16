package database

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountCommonMaterialsUpgradePreservesOldLedgerAndWriter(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old-save.sqlite3")
	db, err := openSQLite(ctx, path, 1, 1000)
	require.NoError(t, err)
	defer db.Close()
	// Construct the preceding schema, including its real migration ledger.
	oldChecksums := map[string]string{}
	for _, name := range sqliteMigrationSections {
		if name == "0039_account_common_materials.sql" {
			continue
		}
		require.NoError(t, migrateSQLite(ctx, db, name))
		query, err := sqliteMigrationSection(name)
		require.NoError(t, err)
		oldChecksums[name] = sqliteMigrationChecksum(query)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO accounts(id,username) VALUES(1,'upgrade-fixture')")
	require.NoError(t, err)
	old := []byte(`{"version":"account-materials-v1","counts":{"363":7},"future":{"keep":true}}`)
	_, err = db.ExecContext(ctx, "INSERT INTO account_material_storage(account_id,counts) VALUES(1,?)", old)
	require.NoError(t, err)
	store, err := Open(ctx, Config{Driver: DriverSQLite, SQLitePath: path})
	require.NoError(t, err)
	defer store.Close()
	var saved []byte
	require.NoError(t, db.QueryRowContext(ctx, "SELECT counts FROM account_material_storage WHERE account_id=1").Scan(&saved))
	require.Equal(t, old, saved)
	for name, checksum := range oldChecksums {
		var after string
		require.NoError(t, db.QueryRowContext(ctx, "SELECT checksum FROM storage_migrations WHERE name=?", name).Scan(&after))
		require.Equal(t, checksum, after, name)
	}
	common := `{"common_items":{"30001":19},"symbol_receipt":{"keep":true}}`
	_, err = db.ExecContext(ctx, "INSERT INTO account_common_material_storage(account_id,data) VALUES(1,?)", common)
	require.NoError(t, err)
	// A preceding binary writes only its old table: common items must survive.
	_, err = db.ExecContext(ctx, "UPDATE account_material_storage SET counts=? WHERE account_id=1", []byte(`{"version":"account-materials-v1","counts":{"363":8}}`))
	require.NoError(t, err)
	merged, err := store.AccountMaterials(ctx, 1)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(merged, &fields))
	require.JSONEq(t, `{"363":8}`, string(fields["counts"]))
	require.JSONEq(t, `{"30001":19}`, string(fields["common_items"]))
	require.JSONEq(t, `{"keep":true}`, string(fields["symbol_receipt"]))
}
