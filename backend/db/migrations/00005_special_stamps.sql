-- +goose Up
create type special_stamp_rarity as enum ('common', 'uncommon', 'rare', 'epic', 'legendary');

-- What a special stamp does when it's applied to a submission. 'none' stamps
-- are only collectibles and can't be applied.
create type special_stamp_effect as enum ('none', 'priority_review');

create table special_stamp_types (
    id bigint generated always as identity primary key,
    name text not null,
    description text not null default '',
    image_url text,
    rarity special_stamp_rarity not null,
    effect special_stamp_effect not null default 'none',
    -- How many can ever be given out; null is unlimited. Lowering it below
    -- issued_count just stops further drops.
    supply_cap integer check (supply_cap >= 0),
    issued_count integer not null default 0 check (issued_count >= 0),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create trigger special_stamp_types_set_updated_at before update on special_stamp_types
    for each row execute function set_updated_at();

create table box_types (
    id bigint generated always as identity primary key,
    name text not null,
    description text not null default '',
    image_url text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create trigger box_types_set_updated_at before update on box_types
    for each row execute function set_updated_at();

-- A box type's loot table. A stamp type's odds are its weight over the total
-- weight of the types that still have supply left.
create table box_drops (
    box_type_id bigint not null references box_types (id) on delete cascade,
    special_stamp_type_id bigint not null references special_stamp_types (id),
    weight integer not null check (weight > 0),
    primary key (box_type_id, special_stamp_type_id)
);

create table milestones (
    id bigint generated always as identity primary key,
    name text not null,
    approved_seconds integer not null unique check (approved_seconds > 0),
    box_type_id bigint not null references box_types (id),
    created_at timestamptz not null default now()
);

create table boxes (
    id bigint generated always as identity primary key,
    user_id bigint not null references users (id),
    box_type_id bigint not null references box_types (id),
    milestone_id bigint references milestones (id),
    granted_by bigint references users (id), -- the admin, when it wasn't from a milestone
    granted_at timestamptz not null default now(),
    opened_at timestamptz,
    unique (user_id, milestone_id),
    check ((milestone_id is null) <> (granted_by is null))
);

create table special_stamps (
    id bigint generated always as identity primary key,
    user_id bigint not null references users (id),
    special_stamp_type_id bigint not null references special_stamp_types (id),
    box_id bigint unique references boxes (id),
    granted_by bigint references users (id), -- the admin, when it didn't come from a box
    -- Set when it's used on a submission, which uses it up. Unique, so a
    -- submission takes at most one special stamp: no stacking.
    submission_id bigint unique references submissions (id),
    acquired_at timestamptz not null default now(),
    check ((box_id is null) <> (granted_by is null))
);

create index on special_stamps (user_id);

-- +goose Down
drop table special_stamps;
drop table boxes;
drop table milestones;
drop table box_drops;
drop table box_types;
drop table special_stamp_types;
drop type special_stamp_effect;
drop type special_stamp_rarity;
