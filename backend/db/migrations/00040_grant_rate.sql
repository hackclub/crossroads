-- +goose Up

-- Grants are $6.50 an approved hour, not $7.50 (the default for new grants
-- is already $6.50, in api/shop.go). Only grants still at $7.50 change, so a
-- rate an admin set to something else stays. Orders already placed keep the
-- price they were bought at.
update shop_items set grant_cents_per_hour = 650 where kind = 'grant' and grant_cents_per_hour = 750;

-- +goose Down
update shop_items set grant_cents_per_hour = 750 where kind = 'grant' and grant_cents_per_hour = 650;
