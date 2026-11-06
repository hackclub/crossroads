-- name: ListTrackedProjects :many
-- Projects whose time can be tracked: they have linked Hackatime projects,
-- and their owner has a Slack ID to look them up by.
select projects.id, users.slack_id::text as slack_id,
    array_agg(project_hackatime_projects.hackatime_project order by project_hackatime_projects.hackatime_project)::text[] as hackatime_projects
from projects
join users on users.id = projects.user_id
join project_hackatime_projects on project_hackatime_projects.project_id = projects.id
where users.slack_id is not null
group by projects.id, users.slack_id
order by projects.id;

-- name: ClearProjectActivity :exec
delete from project_activity
where project_id = $1 and day between sqlc.arg(from_day)::date and sqlc.arg(to_day)::date;

-- name: SetProjectActivity :exec
insert into project_activity (project_id, day, seconds) values ($1, $2, $3)
on conflict (project_id, day) do update set seconds = excluded.seconds, updated_at = now();

-- name: DailyProjectActivity :many
-- Each day from start_day to end_day, including empty ones: how many
-- participants spent at least min_seconds on their projects, how many of
-- their projects got any time, and the total.
with per_user as (
    select project_activity.day, projects.user_id,
        count(*) as projects, sum(project_activity.seconds) as seconds
    from project_activity
    join projects on projects.id = project_activity.project_id
    join users on users.id = projects.user_id
    where users.role = 'participant'
        and project_activity.day between sqlc.arg(start_day)::date and sqlc.arg(end_day)::date
    group by project_activity.day, projects.user_id
)
select days.day::date as day,
    count(per_user.user_id) filter (where per_user.seconds >= sqlc.arg(min_seconds)::integer)::integer as active_participants,
    coalesce(sum(per_user.projects), 0)::integer as active_projects,
    coalesce(sum(per_user.seconds), 0)::bigint as seconds
from generate_series(sqlc.arg(start_day)::date, sqlc.arg(end_day)::date, interval '1 day') as days (day)
left join per_user on per_user.day = days.day::date
group by days.day
order by days.day;

-- name: CountActiveParticipants :one
-- Participants who spent at least min_seconds on their projects on at least
-- one day from start_day to end_day.
select count(distinct active.user_id)::integer from (
    select projects.user_id
    from project_activity
    join projects on projects.id = project_activity.project_id
    join users on users.id = projects.user_id
    where users.role = 'participant'
        and project_activity.day between sqlc.arg(start_day)::date and sqlc.arg(end_day)::date
    group by project_activity.day, projects.user_id
    having sum(project_activity.seconds) >= sqlc.arg(min_seconds)::integer
) as active;

-- name: LastProjectActivityUpdate :one
select updated_at from project_activity order by updated_at desc limit 1;
