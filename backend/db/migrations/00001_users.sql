-- +goose Up

-- Shared by every table with an updated_at column.
-- +goose StatementBegin
create function set_updated_at() returns trigger language plpgsql as $$
begin
    new.updated_at = now();
    return new;
end;
$$;
-- +goose StatementEnd

create type user_role as enum ('participant', 'reviewer', 'admin');

create table users (
    id bigint generated always as identity primary key,
    -- hca_id through ysws_eligible come from Hack Club Auth and are refreshed
    -- every time the person signs in.
    hca_id text not null unique,
    slack_id text unique,
    email text not null,
    name text not null,
    verification_status text not null default 'needs_submission',
    ysws_eligible boolean not null default false,
    role user_role not null default 'participant',
    -- Running total of stamp_ledger. Only the ledger trigger changes it.
    stamps integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create trigger users_set_updated_at before update on users
    for each row execute function set_updated_at();

create table sessions (
    token_hash bytea primary key, -- sha256 of the cookie value
    user_id bigint not null references users (id) on delete cascade,
    created_at timestamptz not null default now(),
    expires_at timestamptz not null
);

create index on sessions (user_id);

-- +goose Down
drop table sessions;
drop table users;
drop type user_role;
drop function set_updated_at;
