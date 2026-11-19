-- +goose Up

-- Slack DMs from Tickets, Please (the Slack app's bot under that name),
-- telling people what happened to their projects (api/dms.go). Each one is
-- written in the same transaction as the change, worded as it was then, and
-- sent soon after; if Slack doesn't take it, it's tried again later.
create type project_dm_kind as enum (
    'approved',        -- signed off, with the tickets it paid
    'rejected',        -- soft or hard, with the feedback
    'reopened',        -- a rejection was undone; it's back in review
    'resubmit_allowed' -- a hard-rejected project can be shipped again
);

create table project_dms (
    id bigint generated always as identity primary key,
    user_id bigint not null references users (id),
    project_id bigint not null references projects (id),
    submission_id bigint references submissions (id), -- none for resubmit_allowed
    kind project_dm_kind not null,
    text text not null, -- Slack mrkdwn
    created_at timestamptz not null default now(),
    -- Sending: how many tries, the last one's error, and when to try next.
    attempts integer not null default 0,
    error text,
    next_try_at timestamptz not null default now(),
    -- Where it went: their DM with the app, or the test channel from any
    -- build but the real site.
    sent_at timestamptz,
    channel_id text,
    message_ts text,
    -- Given up on: they have no Slack ID, Slack can't deliver it, or it's
    -- waited too long to still be news. error says which.
    abandoned_at timestamptz,
    constraint project_dms_sent check (num_nulls(sent_at, channel_id, message_ts) in (0, 3)),
    constraint project_dms_sent_or_abandoned check (sent_at is null or abandoned_at is null)
);

create index project_dms_to_send on project_dms (next_try_at)
    where sent_at is null and abandoned_at is null;

-- +goose Down
drop table project_dms;
drop type project_dm_kind;
