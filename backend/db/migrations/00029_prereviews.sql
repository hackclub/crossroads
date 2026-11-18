-- +goose Up

-- An AI pre-review of a ship: what Claude made of it before a reviewer
-- looks, pasted in from a prompt the site writes or uploaded by the laptop
-- script (scripts/prereview.mjs). One per ship; a newer one replaces it.
-- It's never a review: no ship_events, no pay.
create table ship_prereviews (
    submission_id bigint primary key references submissions(id) on delete cascade,
    result jsonb not null,
    source text not null check (source in ('paste', 'script')),
    model text,
    created_by bigint references users(id) on delete set null,
    created_at timestamptz not null default now()
);

-- +goose Down

drop table ship_prereviews;
