-- +goose Up

-- Grants: money for a project, sold in the Smugglers' Emporium in $5 pieces
-- at a rate per approved hour. Postgres can't use a new enum value in the
-- transaction that adds it, so the rest of the Emporium is the next migration.
alter type shop_item_kind add value if not exists 'grant';

-- +goose Down

-- Postgres can't drop a value from an enum. Left in, it's harmless, and
-- migration 3's down drops the whole type.
