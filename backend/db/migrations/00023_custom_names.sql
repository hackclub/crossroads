-- +goose Up

-- People can change the name the site shows (clicking it in the top bar).
-- Signing in copies their name from Hack Club Auth, so this marks a name they
-- chose, which signing in leaves alone. first_name and last_name stay their
-- legal names from Hack Club Auth, for the Unified YSWS DB and fulfilment.
alter table users add column name_customized boolean not null default false;

-- +goose Down
alter table users drop column name_customized;
