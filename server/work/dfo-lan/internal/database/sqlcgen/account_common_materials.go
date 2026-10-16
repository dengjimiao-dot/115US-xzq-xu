package sqlcgen

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// Retained internal query/type layer; SQLite is the active database engine.
func (q *Queries) AccountCommonMaterials(ctx context.Context, account int64) (json.RawMessage, error) {
	var raw []byte
	e := q.db.QueryRowContext(ctx, "SELECT data FROM account_common_material_storage WHERE account_id=?", account).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	return json.RawMessage(raw), e
}
func (q *Queries) SaveAccountCommonMaterials(ctx context.Context, account int64, raw json.RawMessage) error {
	_, e := q.db.ExecContext(ctx, "INSERT INTO account_common_material_storage(account_id,data) VALUES(?,?) ON CONFLICT(account_id) DO UPDATE SET data=excluded.data", account, string(raw))
	return e
}
