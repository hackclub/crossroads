-- +goose Up

-- Roles set by email for people who haven't signed in yet. The first time
-- someone signs in with that email, they get the role and the invite is used
-- up.
create table role_invites (
    id bigint generated always as identity primary key,
    email text not null unique check (email = lower(email)),
    role user_role not null check (role <> 'participant'),
    invited_by bigint references users (id) on delete set null,
    created_at timestamptz not null default now()
);

-- +goose Down
drop table role_invites;
