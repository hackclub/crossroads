-- +goose Up

-- A price lock fixes someone's ticket at the price of the batch on sale when
-- they buy it. Postgres can't use a new enum value in the transaction that
-- adds it, so the rest of ticket batches is the next migration.
alter type shop_item_kind add value if not exists 'price_lock';

-- +goose Down

-- Postgres can't drop a value from an enum. Left in, it's harmless, and
-- migration 3's down drops the whole type.
