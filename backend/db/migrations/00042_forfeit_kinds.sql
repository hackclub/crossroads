-- +goose Up

-- A hard reject forfeits the tickets its project earned ('forfeit'), and
-- undoing it can give them back ('restore'). Postgres can't use a new enum
-- value in the transaction that adds it, so the rest is the next migration.
alter type stamp_entry_kind add value if not exists 'forfeit';
alter type stamp_entry_kind add value if not exists 'restore';

-- +goose Down
-- Postgres can't drop a value from an enum. Left in, they're harmless, and
-- migration 4's down drops the whole type.
