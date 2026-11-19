-- +goose Up

-- What the maker tells reviewers when they ship: a test login, what changed
-- since the last ship, what to look at. Only they and reviewers see it.
alter table submissions add column notes_for_reviewers text;

-- +goose Down
alter table submissions drop column notes_for_reviewers;
