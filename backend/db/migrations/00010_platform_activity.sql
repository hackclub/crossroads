-- +goose Up

-- Which UTC days each person used the platform: a row for each day they
-- made a signed-in request. activity.Visits adds it on their first request
-- of the day.
create table platform_activity (
    user_id bigint not null references users (id) on delete cascade,
    day date not null,
    primary key (user_id, day)
);

create index on platform_activity (day);

-- Sign-ins from before this table count as visits on their day.
insert into platform_activity (user_id, day)
select distinct user_id, (created_at at time zone 'UTC')::date from sessions;

-- +goose Down

drop table platform_activity;
