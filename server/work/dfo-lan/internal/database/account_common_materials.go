package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type accountCommonMaterialQueries interface {
	AccountCommonMaterials(context.Context, int64) (json.RawMessage, error)
	SaveAccountCommonMaterials(context.Context, int64, json.RawMessage) error
}

func (q *sqliteQueries) AccountCommonMaterials(ctx context.Context, account int64) (json.RawMessage, error) {
	var raw []byte
	e := q.queryRow(ctx, "SELECT data FROM account_common_material_storage WHERE account_id=?", account).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	return json.RawMessage(raw), e
}
func (q *sqliteQueries) SaveAccountCommonMaterials(ctx context.Context, account int64, raw json.RawMessage) error {
	if !json.Valid(raw) {
		return fmt.Errorf("invalid common materials JSON")
	}
	_, e := q.exec(ctx, "INSERT INTO account_common_material_storage(account_id,data) VALUES(?,?) ON CONFLICT(account_id) DO UPDATE SET data=excluded.data", account, string(raw))
	return e
}
func mergeAccountCommonMaterials(ctx context.Context, q querySet, account int64, raw json.RawMessage) (json.RawMessage, error) {
	extra, e := q.AccountCommonMaterials(ctx, account)
	if e != nil {
		return nil, e
	}
	if len(extra) == 0 {
		return raw, nil
	}
	fields := map[string]json.RawMessage{}
	if len(raw) > 0 && string(raw) != "null" {
		if e = json.Unmarshal(raw, &fields); e != nil {
			return nil, e
		}
	} else {
		fields["version"] = json.RawMessage(`"account-materials-v1"`)
	}
	common := map[string]json.RawMessage{}
	if e = json.Unmarshal(extra, &common); e != nil {
		return nil, e
	}
	for _, key := range []string{"common_items", "symbol_receipt"} {
		if value, ok := common[key]; ok {
			fields[key] = value
		}
	}
	return json.Marshal(fields)
}

// Keep the old document shape unchanged; new data has its own durable table.
func saveAccountCommonMaterials(ctx context.Context, q querySet, account int64, raw json.RawMessage) (json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	if e := json.Unmarshal(raw, &fields); e != nil {
		return nil, e
	}
	extra := map[string]json.RawMessage{}
	for _, key := range []string{"common_items", "symbol_receipt"} {
		if value, ok := fields[key]; ok {
			extra[key] = value
			delete(fields, key)
		}
	}
	if len(extra) > 0 {
		value, e := json.Marshal(extra)
		if e != nil {
			return nil, e
		}
		if e = q.SaveAccountCommonMaterials(ctx, account, value); e != nil {
			return nil, e
		}
	}
	return json.Marshal(fields)
}
