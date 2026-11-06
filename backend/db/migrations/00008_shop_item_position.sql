-- +goose Up

-- Where an item sits in the shop: lower comes first, ties go cheapest first.
-- Lets admins put the most wanted things at the top.
alter table shop_items add column position integer not null default 0;

-- +goose Down

alter table shop_items drop column position;
