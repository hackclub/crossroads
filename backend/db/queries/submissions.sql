-- name: ListSubmissions :many
select * from submissions where project_id = $1 order by id desc;

-- name: ListSubmissionsForUser :many
select submissions.* from submissions
join projects on projects.id = submissions.project_id
where projects.user_id = $1
order by submissions.id desc;

-- name: CreateSubmission :one
insert into submissions (project_id, hackatime_projects, hackatime_seconds, new_seconds)
values ($1, $2, $3, $4)
returning *;

-- name: ClaimedSecondsForUser :one
-- Time the user has already claimed: everything their pending and approved
-- submissions counted, including time a reviewer didn't accept.
select coalesce(sum(submissions.new_seconds), 0)::bigint
from submissions
join projects on projects.id = submissions.project_id
where projects.user_id = $1 and submissions.status in ('pending', 'approved');
