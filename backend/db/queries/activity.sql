-- name: ListTrackedProjects :many
-- Projects whose time can be tracked: they have linked Hackatime projects,
-- and their owner has a Slack ID to look them up by.
select projects.id, users.slack_id::text as slack_id, projects.hours_start,
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

-- name: DailyActivity :many
-- Each day from start_day to end_day, including empty ones: how many
-- participants spent at least min_seconds on their projects, how many of
-- their projects got any time, and the total; and how many participants
-- used the platform, how many signed up, how many there were by the end
-- of the day, and the hours they shipped.
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
    coalesce(sum(per_user.seconds), 0)::bigint as seconds,
    (select count(*) from platform_activity
        join users on users.id = platform_activity.user_id
        where platform_activity.day = days.day::date and users.role = 'participant')::integer as visitors,
    (select count(*) from users
        where (users.created_at at time zone 'UTC')::date = days.day::date and users.role = 'participant')::integer as sign_ups,
    (select count(*) from users
        where (users.created_at at time zone 'UTC')::date <= days.day::date and users.role = 'participant')::integer as participants,
    -- The hours participants shipped that day, whatever became of them.
    (select coalesce(sum(submissions.new_seconds), 0) from submissions
        join projects on projects.id = submissions.project_id
        join users on users.id = projects.user_id
        where (submissions.submitted_at at time zone 'UTC')::date = days.day::date and users.role = 'participant')::bigint as shipped_seconds
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

-- name: RecordVisit :exec
insert into platform_activity (user_id, day) values ($1, $2)
on conflict do nothing;

-- name: CountVisitors :one
-- Participants who used the platform on at least one day from start_day to
-- end_day.
select count(distinct platform_activity.user_id)::integer
from platform_activity
join users on users.id = platform_activity.user_id
where users.role = 'participant'
    and platform_activity.day between sqlc.arg(start_day)::date and sqlc.arg(end_day)::date;

-- name: CommunitySeconds :one
-- Hours shipped by all participants from starts_at to ends_at, for the
-- community goal on the home page. A ship counts as soon as it's submitted,
-- without waiting for review, and drops out again if it's rejected.
select coalesce(sum(submissions.new_seconds), 0)::bigint
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
where users.role = 'participant'
    and submissions.status <> 'rejected'
    and submissions.submitted_at >= sqlc.arg(starts_at)
    and submissions.submitted_at <= sqlc.arg(ends_at);

-- name: HoursFlow :one
-- Where every hour participants logged has gone, all time, in hours and in
-- ships (the admin Stats tab's Sankey). A soft-rejected ship whose project
-- was shipped again is left out: the newer ship claims its hours again. A
-- ship is in one place at a time; cut hours (approved below what the ship
-- claimed) leave the ship where it is, so they count the ships they affect.
with ships as (
    select submissions.status, submissions.new_seconds, submissions.approved_seconds, submissions.unified_submitted_at,
        coalesce((
            select e.blocks_resubmit from ship_events e
            where e.submission_id = submissions.id and e.kind = 'rejection'
            order by e.id desc limit 1
        ), false) as hard
    from submissions
    join projects on projects.id = submissions.project_id
    join users on users.id = projects.user_id
    where users.role = 'participant'
        and not (submissions.status = 'rejected' and exists (
            select 1 from submissions later
            where later.project_id = submissions.project_id and later.id > submissions.id
        ))
)
select
    (select coalesce(sum(project_activity.seconds), 0) from project_activity
        join projects on projects.id = project_activity.project_id
        join users on users.id = projects.user_id
        where users.role = 'participant')::bigint as logged_seconds,
    count(*)::bigint as shipped_ships,
    coalesce(sum(new_seconds), 0)::bigint as shipped_seconds,
    count(*) filter (where status = 'pending')::bigint as review_ships,
    coalesce(sum(new_seconds) filter (where status = 'pending'), 0)::bigint as review_seconds,
    count(*) filter (where status = 'fraud_check')::bigint as fraud_ships,
    coalesce(sum(new_seconds) filter (where status = 'fraud_check'), 0)::bigint as fraud_seconds,
    count(*) filter (where status = 'awaiting_hq')::bigint as hq_ships,
    coalesce(sum(new_seconds) filter (where status = 'awaiting_hq'), 0)::bigint as hq_seconds,
    count(*) filter (where status = 'rejected' and not hard)::bigint as soft_rejected_ships,
    coalesce(sum(new_seconds) filter (where status = 'rejected' and not hard), 0)::bigint as soft_rejected_seconds,
    count(*) filter (where status = 'rejected' and hard)::bigint as hard_rejected_ships,
    coalesce(sum(new_seconds) filter (where status = 'rejected' and hard), 0)::bigint as hard_rejected_seconds,
    count(*) filter (where status = 'approved')::bigint as signed_off_ships,
    coalesce(sum(new_seconds) filter (where status = 'approved'), 0)::bigint as signed_off_seconds,
    count(*) filter (where status = 'approved' and approved_seconds < new_seconds)::bigint as cut_ships,
    coalesce(sum(new_seconds - approved_seconds) filter (where status = 'approved' and approved_seconds < new_seconds), 0)::bigint as cut_seconds,
    -- Approving more than Hackatime tracked (with a reason) adds hours.
    coalesce(sum(approved_seconds - new_seconds) filter (where status = 'approved' and approved_seconds > new_seconds), 0)::bigint as added_seconds,
    coalesce(sum(approved_seconds) filter (where status = 'approved'), 0)::bigint as approved_seconds,
    count(*) filter (where status = 'approved' and unified_submitted_at is not null)::bigint as unified_ships,
    coalesce(sum(approved_seconds) filter (where status = 'approved' and unified_submitted_at is not null), 0)::bigint as unified_seconds
from ships;

-- name: ParticipantFunnel :one
-- How far participants have got, all time: each counted at every step they
-- reached (the admin Stats tab, like Stardance's user funnel). Logged time is
-- the activity poller's, on linked projects; the rail pass is the shop item
-- of kind 'ticket', in any order not cancelled.
select
    count(*)::bigint as signed_up,
    count(*) filter (where exists (
        select 1 from projects p where p.user_id = u.id
    ))::bigint as made_project,
    count(*) filter (where exists (
        select 1 from projects p join project_hackatime_projects h on h.project_id = p.id where p.user_id = u.id
    ))::bigint as linked_hackatime,
    count(*) filter (where exists (
        select 1 from projects p join project_activity a on a.project_id = p.id where p.user_id = u.id and a.seconds > 0
    ))::bigint as logged_time,
    count(*) filter (where exists (
        select 1 from projects p join submissions s on s.project_id = p.id where p.user_id = u.id
    ))::bigint as shipped,
    count(*) filter (where exists (
        select 1 from projects p join submissions s on s.project_id = p.id where p.user_id = u.id and s.status = 'approved'
    ))::bigint as approved,
    count(*) filter (where (
        select coalesce(sum(s.approved_seconds), 0) from projects p join submissions s on s.project_id = p.id
        where p.user_id = u.id and s.status = 'approved'
    ) >= sqlc.arg(goal_seconds)::bigint)::bigint as reached_goal,
    count(*) filter (where exists (
        select 1 from orders o join shop_items i on i.id = o.shop_item_id
        where o.user_id = u.id and i.kind = 'ticket' and o.status <> 'cancelled'
    ))::bigint as rail_pass
from users u
where u.role = 'participant';
