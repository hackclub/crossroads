-- +goose Up

-- Clicks on the buttons of messages posted with the Slack bot (slackbot/),
-- by Slack ID, so they count people who haven't signed in too. Join on
-- users.slack_id to see who has.
create table slack_clicks (
    id bigint generated always as identity primary key,
    slack_id text not null,
    button text not null,
    channel_id text,
    message_ts text,
    clicked_at timestamptz not null default now()
);

create index on slack_clicks (button, clicked_at);

-- +goose Down
drop table slack_clicks;
