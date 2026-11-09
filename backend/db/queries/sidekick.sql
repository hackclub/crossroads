-- Queries for the Sidekick protocol (api/sidekick.go).

-- name: ListProjectsForSidekick :many
-- Projects with at least one ship in one of statuses, the ones waiting
-- longest first (priority review before everything), one page at a time.
select sqlc.embed(projects), sqlc.embed(users), count(*) over () as total
from projects
join users on users.id = projects.user_id
where exists (
    select 1 from submissions s
    where s.project_id = projects.id and s.status::text = any(sqlc.arg(statuses)::text[])
)
order by
    exists (
        select 1 from special_stamps
        join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
        join submissions s on s.id = special_stamps.submission_id
        where s.project_id = projects.id
            and s.status::text = any(sqlc.arg(statuses)::text[])
            and special_stamp_types.effect = 'priority_review'
    ) desc,
    (select min(s.submitted_at) from submissions s
        where s.project_id = projects.id and s.status::text = any(sqlc.arg(statuses)::text[])),
    projects.id
limit sqlc.arg(page_size) offset sqlc.arg(page_offset);

-- name: GetProjectWithOwner :one
select sqlc.embed(projects), sqlc.embed(users)
from projects
join users on users.id = projects.user_id
where projects.id = $1;

-- name: ListProjectsWithOwnerByUser :many
select sqlc.embed(projects), sqlc.embed(users)
from projects
join users on users.id = projects.user_id
where projects.user_id = $1 and projects.id <> sqlc.arg(exclude_project_id)
    and exists (select 1 from submissions s where s.project_id = projects.id)
order by projects.id;

-- name: ListSubmissionsForProjects :many
select * from submissions where project_id = any(sqlc.arg(project_ids)::bigint[])
order by project_id, id;

-- name: ListHackatimeLinksForProjects :many
select project_id, hackatime_project from project_hackatime_projects
where project_id = any(sqlc.arg(project_ids)::bigint[])
order by project_id, hackatime_project;

-- name: PriorityShipIDs :many
-- Which of these ships have a priority review stamp on them.
select special_stamps.submission_id::bigint as submission_id from special_stamps
join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
where special_stamps.submission_id = any(sqlc.arg(submission_ids)::bigint[])
    and special_stamp_types.effect = 'priority_review';

-- name: SidekickStats :one
select
    (select count(distinct project_id) from submissions where status = 'pending') as pending_review,
    (select count(*) from submissions where status in ('fraud_check', 'awaiting_hq')) as pending_hq;
