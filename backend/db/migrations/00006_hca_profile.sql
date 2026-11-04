-- +goose Up

-- More of what Hack Club Auth knows, now that the app can ask for the
-- basic_info and address scopes. Like the rest, it's refreshed on every
-- sign-in. The Unified YSWS DB needs all of it.
alter table users
    add column first_name text not null default '',
    add column last_name text not null default '',
    add column birthday date,
    add column phone_number text,
    -- Their primary mailing address, shaped like db.Address.
    add column address jsonb;

-- +goose Down
alter table users
    drop column address,
    drop column phone_number,
    drop column birthday,
    drop column last_name,
    drop column first_name;
