-- name: ListReviewQueue :many
-- Pending submissions, priority review first, then oldest first.
select
    submissions.id,
    submissions.project_id,
    submissions.hackatime_projects,
    submissions.new_seconds,
    submissions.submitted_at,
    projects.name as project_name,
    users.id as user_id,
    users.name as user_name,
    users.slack_id,
    exists (
        select 1 from special_stamps
        join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
        where special_stamps.submission_id = submissions.id and special_stamp_types.effect = 'priority_review'
    ) as priority
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
where submissions.status = 'pending'
order by priority desc, submissions.submitted_at, submissions.id;

-- name: GetSubmissionForReview :one
select sqlc.embed(submissions), sqlc.embed(projects), sqlc.embed(users)
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
where submissions.id = $1;

-- name: LockSubmission :one
select * from submissions where id = $1 for update;

-- name: ReviewSubmission :one
update submissions set
    status = $2,
    approved_seconds = $3,
    feedback = $4,
    internal_notes = $5,
    reviewer_id = $6,
    reviewed_at = now()
where id = $1 and status = 'pending'
returning *;

-- name: LockUser :one
select * from users where id = $1 for update;

-- name: ApprovedSecondsForUser :one
select coalesce(sum(submissions.approved_seconds), 0)::bigint
from submissions
join projects on projects.id = submissions.project_id
where projects.user_id = $1 and submissions.status = 'approved';

-- name: GrantMilestoneBoxes :many
-- Grants a box for every milestone at or under approved_seconds that the user
-- doesn't already have one for.
insert into boxes (user_id, box_type_id, milestone_id)
select $1, milestones.box_type_id, milestones.id
from milestones
where milestones.approved_seconds <= sqlc.arg(approved_seconds)::integer
on conflict (user_id, milestone_id) do nothing
returning *;

-- name: ListProjectsSharingLinks :many
-- Other projects, anyone's, with the same repo or demo link as these, ignoring
-- the scheme, www., case, trailing slashes and a .git suffix.
select projects.id, projects.name, projects.repo_url, projects.demo_url, users.id as user_id, users.name as user_name
from projects
join users on users.id = projects.user_id
where projects.id <> sqlc.arg(project_id)
    and (
        regexp_replace(lower(projects.repo_url), '^https?://(www\.)?|(\.git)?/*$', '', 'g')
            = regexp_replace(lower(sqlc.narg(repo_url)::text), '^https?://(www\.)?|(\.git)?/*$', '', 'g')
        or regexp_replace(lower(projects.demo_url), '^https?://(www\.)?|/*$', '', 'g')
            = regexp_replace(lower(sqlc.narg(demo_url)::text), '^https?://(www\.)?|/*$', '', 'g')
    )
order by projects.id;
