-- +goose Up

-- How much time each project got on each UTC day, on the Hackatime projects
-- linked to it. The activity poller (package activity) keeps it up to date
-- from Hackatime's heartbeat spans; a day with no time has no row. Daily
-- active participants are the people whose projects have time on a day.
create table project_activity (
    project_id bigint not null references projects (id) on delete cascade,
    day date not null,
    seconds integer not null check (seconds > 0),
    updated_at timestamptz not null default now(),
    primary key (project_id, day)
);

create index on project_activity (day);

-- +goose Down
drop table project_activity;
