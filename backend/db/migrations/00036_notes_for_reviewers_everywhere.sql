-- +goose Up

-- Notes for reviewers is migration 34, but the dev database had already
-- given 34 to item_regions, so it skipped them. Everywhere else has the
-- column already.
alter table submissions add column if not exists notes_for_reviewers text;

-- +goose Down
-- Nothing to undo: 34's Down drops the column.
