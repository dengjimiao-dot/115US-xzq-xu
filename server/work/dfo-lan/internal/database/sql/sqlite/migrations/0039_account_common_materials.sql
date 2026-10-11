-- Additive storage: older binaries update account_material_storage without
-- touching this table, so rollback cannot erase the new account-common items.
CREATE TABLE IF NOT EXISTS account_common_material_storage (
 account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
 data TEXT NOT NULL CHECK(json_valid(data))
);
