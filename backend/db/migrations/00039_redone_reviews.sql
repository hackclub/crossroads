-- +goose Up

-- A first-stage decision by the reviewer a ship was sent back to, to change
-- their review (migration 37). It's a redo of a review they've already made,
-- so reviewer pay (migration 38) doesn't pay for it again. A different
-- reviewer picking the ship up is a new review, and isn't marked.
alter table ship_events add column redo boolean not null default false;

-- +goose Down
alter table ship_events drop column redo;
