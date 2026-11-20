-- +goose Up

-- The reviewer a fraud check or HQ sent a ship back to, to change their
-- review. Null for every other ship. The ship waits in that reviewer's
-- "Changes requested" queue, and only they get it from "Next", until it's
-- decided at the first stage again. Why it came back is on the sent_back
-- ship event.
alter table submissions add column returned_to bigint references users (id);

-- +goose Down
alter table submissions drop column returned_to;
