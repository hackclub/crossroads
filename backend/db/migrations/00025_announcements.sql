-- +goose Up

-- Announcements superadmins write on the admin page's Announce tab and post
-- to the station channel with the Slack app's bot (api/announcements.go): a
-- draft until it's posted, and kept after it's deleted from Slack, so its
-- numbers are too. Clicks on its buttons are the slack_clicks rows with its
-- channel_id and message_ts.
create table announcements (
    id bigint generated always as identity primary key,
    -- The editor's own copy of the content, to open it again. Only the
    -- platform reads it.
    doc jsonb not null,
    -- What goes to Slack: Block Kit blocks and the notification text.
    blocks jsonb not null,
    text text not null,
    -- The buttons in blocks, worked out whenever it's saved:
    -- [{action_id, text, url, special_stamp_type_id}], the last only for a
    -- claim link.
    buttons jsonb not null default '[]',
    -- Whether it @channels: the blocks have a broadcast element.
    pings boolean not null default false,
    created_by bigint not null references users (id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    posted_by bigint references users (id),
    posted_at timestamptz,
    channel_id text,
    message_ts text,
    permalink text,
    deleted_at timestamptz, -- deleted from Slack
    constraint announcements_posted check (
        num_nulls(posted_by, posted_at, channel_id, message_ts) in (0, 4)),
    constraint announcements_deleted_once_posted check (deleted_at is null or posted_at is not null),
    unique (channel_id, message_ts)
);

create trigger announcements_set_updated_at before update on announcements
    for each row execute function set_updated_at();

-- An announcement's clicks.
create index on slack_clicks (channel_id, message_ts);

-- +goose Down
drop index slack_clicks_channel_id_message_ts_idx;
drop table announcements;
