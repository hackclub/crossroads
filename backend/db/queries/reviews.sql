-- name: ListPastReviews :many
-- Ships that have been decided (approved or rejected), the latest decided
-- first. With decided_by, only ships that person made a decision on; with
-- status, only ships with that status.
select
    submissions.id,
    submissions.project_id,
    submissions.status,
    submissions.new_seconds,
    submissions.approved_seconds,
    submissions.submitted_at,
    coalesce(submissions.hq_reviewed_at, submissions.fraud_reviewed_at, submissions.reviewed_at)::timestamptz as decided_at,
    submissions.feedback,
    projects.name as project_name,
    (projects.resubmit_blocked_at is not null)::boolean as resubmit_blocked,
    case when projects.resubmit_blocked_at is not null then (
        select coalesce(-sum(stamp_ledger.amount), 0)
        from stamp_ledger
        join submissions shipped on shipped.id = stamp_ledger.submission_id
        where shipped.project_id = projects.id and stamp_ledger.kind in ('forfeit', 'restore')
    ) else 0 end::bigint as forfeited_stamps,
    users.id as user_id,
    users.name as user_name,
    coalesce(decider.name, '')::text as decided_by_name,
    exists (
        select 1 from submissions newer
        where newer.project_id = submissions.project_id and newer.id > submissions.id
    ) as shipped_again
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
left join users decider on decider.id = coalesce(submissions.hq_reviewer_id, submissions.fraud_reviewer_id, submissions.reviewer_id)
where submissions.status in ('approved', 'rejected')
    and (sqlc.narg(status)::submission_status is null or submissions.status = sqlc.narg(status))
    and (sqlc.narg(decided_by)::bigint is null or exists (
        select 1 from ship_events d
        where d.submission_id = submissions.id and d.actor_id = sqlc.narg(decided_by)
            and d.kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')
    ))
order by decided_at desc, submissions.id desc
limit sqlc.arg(max_rows);

-- name: NewerSubmissionExists :one
select exists (select 1 from submissions where project_id = $1 and id > $2);

-- name: ListReviewQueue :many
-- Ships at one stage, priority review first, then oldest first. Claims are
-- returned as they are; expired ones are ignored in Go.
select
    submissions.id,
    submissions.project_id,
    submissions.status,
    submissions.hackatime_projects,
    submissions.new_seconds,
    submissions.approved_seconds,
    submissions.submitted_at,
    submissions.reviewed_at,
    submissions.fraud_reasons,
    submissions.override_reason,
    submissions.claimed_by,
    submissions.claim_expires_at,
    projects.name as project_name,
    users.id as user_id,
    users.name as user_name,
    users.slack_id,
    coalesce(reviewer.name, '')::text as reviewer_name,
    coalesce(claimer.name, '')::text as claimed_by_name,
    submissions.returned_to,
    coalesce(returned_to.name, '')::text as returned_to_name,
    -- Who sent it back and why: its latest sent_back event.
    coalesce(back.actor_name, '')::text as returned_by_name,
    coalesce(back.stage::text, '')::text as returned_from,
    back.internal as returned_note,
    exists (
        select 1 from special_stamps
        join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
        where special_stamps.submission_id = submissions.id and special_stamp_types.effect = 'priority_review'
    ) as priority,
    exists (
        select 1 from review_skips
        where review_skips.submission_id = submissions.id
            and review_skips.user_id = sqlc.arg(viewer_id)
            and review_skips.skipped_at > now() - interval '3 hours'
    ) as skipped
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
left join users reviewer on reviewer.id = submissions.reviewer_id
left join users claimer on claimer.id = submissions.claimed_by
left join users returned_to on returned_to.id = submissions.returned_to
left join lateral (
    select ship_events.stage, ship_events.internal, actor.name as actor_name
    from ship_events
    left join users actor on actor.id = ship_events.actor_id
    where ship_events.submission_id = submissions.id and ship_events.kind = 'sent_back'
        and submissions.returned_to is not null
    order by ship_events.id desc limit 1
) back on true
where submissions.status = sqlc.arg(status)
order by priority desc, submissions.submitted_at, submissions.id;

-- name: CountOpenSubmissions :many
select status, count(*) as count from submissions
where status in ('pending', 'fraud_check', 'awaiting_hq')
group by status;

-- name: ClaimNextSubmission :one
-- Claims the next ship at a stage for a reviewer: their own live claim if they
-- have one, then ships sent back to them to change their review, otherwise
-- the first one nobody's holding that isn't theirs, wasn't sent back to
-- someone else, and that they haven't skipped lately, priority review
-- first, then oldest first.
-- SKIP LOCKED keeps two reviewers asking at once from getting the same ship.
update submissions set claimed_by = sqlc.arg(user_id), claim_expires_at = now() + interval '30 minutes'
where submissions.id = (
    select s.id from submissions s
    join projects p on p.id = s.project_id
    where s.status = sqlc.arg(status)
        and p.user_id <> sqlc.arg(user_id)
        and (s.claimed_by is null or s.claimed_by = sqlc.arg(user_id) or s.claim_expires_at <= now())
        and (s.returned_to is null or s.returned_to = sqlc.arg(user_id))
        and not exists (
            select 1 from review_skips k
            where k.submission_id = s.id and k.user_id = sqlc.arg(user_id)
                and k.skipped_at > now() - interval '3 hours'
        )
    order by
        coalesce(s.claimed_by = sqlc.arg(user_id) and s.claim_expires_at > now(), false) desc,
        s.returned_to is not null desc,
        exists (
            select 1 from special_stamps
            join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
            where special_stamps.submission_id = s.id and special_stamp_types.effect = 'priority_review'
        ) desc,
        s.submitted_at, s.id
    limit 1
    for update of s skip locked
)
returning *;

-- name: ClaimSubmission :one
-- Claims (or renews) a ship for a reviewer, unless someone else holds it.
update submissions set claimed_by = sqlc.arg(user_id), claim_expires_at = now() + interval '30 minutes'
where id = sqlc.arg(id)
    and status in ('pending', 'fraud_check', 'awaiting_hq')
    and (claimed_by is null or claimed_by = sqlc.arg(user_id) or claim_expires_at <= now())
returning *;

-- name: ReleaseClaim :execrows
update submissions set claimed_by = null, claim_expires_at = null
where id = sqlc.arg(id) and (claimed_by = sqlc.arg(user_id) or sqlc.arg(anyone)::boolean);

-- name: SkipSubmission :exec
insert into review_skips (user_id, submission_id) values ($1, $2)
on conflict (user_id, submission_id) do update set skipped_at = now();

-- name: GetSubmissionForReview :one
select sqlc.embed(submissions), sqlc.embed(projects), sqlc.embed(users)
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
where submissions.id = $1;

-- name: LockSubmission :one
select * from submissions where id = $1 for update;

-- name: SaveSubmissionReview :one
-- Writes a ship's whole review state, worked out in Go (api/stages.go), and
-- drops any claim on it: deciding is the end of a reviewer's turn.
update submissions set
    status = $2,
    approved_seconds = $3,
    feedback = $4,
    internal_notes = $5,
    reviewer_id = $6,
    reviewed_at = $7,
    checks = $8,
    fraud_reasons = $9,
    fraud_reviewer_id = $10,
    fraud_reviewed_at = $11,
    hq_reviewer_id = $12,
    hq_reviewed_at = $13,
    override_reason = $14,
    technical_features = $15,
    deflation_reason = $16,
    returned_to = $17,
    claimed_by = null,
    claim_expires_at = null
where id = $1
returning *;

-- name: LockUser :one
select * from users where id = $1 for update;

-- name: ApprovedSecondsForUser :one
-- Only signed-off ships count, and none on a hard-rejected project: they're
-- forfeit.
select coalesce(sum(submissions.approved_seconds), 0)::bigint
from submissions
join projects on projects.id = submissions.project_id
where projects.user_id = $1 and submissions.status = 'approved' and projects.resubmit_blocked_at is null;

-- name: GrantMilestoneBoxes :many
-- Grants a box for every milestone at or under approved_seconds that the user
-- doesn't already have one for.
insert into boxes (user_id, box_type_id, milestone_id)
select $1, milestones.box_type_id, milestones.id
from milestones
where not milestones.per_project and milestones.approved_seconds <= sqlc.arg(approved_seconds)::integer
on conflict (user_id, milestone_id) where project_id is null do nothing
returning *;

-- name: ApprovedSecondsForProject :one
-- Only signed-off ships count.
select coalesce(sum(approved_seconds), 0)::bigint
from submissions
where project_id = $1 and status = 'approved';

-- name: GrantProjectMilestoneBoxes :many
-- Grants a box with the project for every per-project milestone at or under
-- approved_seconds that it doesn't already have one for.
insert into boxes (user_id, box_type_id, milestone_id, project_id)
select sqlc.arg(user_id), milestones.box_type_id, milestones.id, sqlc.arg(project_id)::bigint
from milestones
where milestones.per_project and milestones.approved_seconds <= sqlc.arg(approved_seconds)::integer
on conflict (project_id, milestone_id) where project_id is not null do nothing
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

-- name: ListProjectDays :many
-- The project's Hackatime time per UTC day on or after since, from the
-- activity poller. Daily totals only.
select day, seconds from project_activity
where project_id = $1 and day >= sqlc.arg(since)::date
order by day;

-- name: AddShipEvent :one
insert into ship_events (submission_id, kind, actor_id, stage, seconds, message, internal, deflation_reason, blocks_resubmit, redo)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
returning *;

-- name: ListProjectShipEvents :many
-- A project's whole timeline, oldest first, with who did each thing.
select sqlc.embed(ship_events), actor.name as actor_name, actor.slack_id as actor_slack_id, actor.hca_id as actor_hca_id
from ship_events
join submissions on submissions.id = ship_events.submission_id
left join users actor on actor.id = ship_events.actor_id
where submissions.project_id = $1
order by ship_events.id;

-- name: LockLatestShipEvent :one
-- A reviewer's latest event of a kind on a ship, for editing it.
select * from ship_events
where submission_id = $1 and kind = $2 and actor_id = $3
order by id desc
limit 1
for update;

-- name: EditShipEvent :one
update ship_events set seconds = $2, message = $3, internal = $4, edited_at = now()
where id = $1
returning *;

-- name: SetSubmissionMessages :exec
-- Keeps the ship's current feedback and notes in step with an edited decision.
update submissions set approved_seconds = $2, feedback = $3, internal_notes = $4 where id = $1;

-- name: GetReviewSettings :one
select * from review_settings;

-- name: UpdateReviewSettings :one
update review_settings set
    fraud_check_everything = $1,
    large_ship_seconds = $2,
    long_day_seconds = $3,
    prereviews_for_reviewers = $4
returning *;

-- name: SetReviewerNote :one
update users set reviewer_note = $2 where id = $1
returning *;

-- name: GetUserByActorID :one
-- A Sidekick actor ID is a Slack ID (U…) or a Hack Club Auth identity ID
-- (ident!…).
select * from users where slack_id = sqlc.arg(actor_id)::text or hca_id = sqlc.arg(actor_id)::text
order by (hca_id = sqlc.arg(actor_id)::text) desc
limit 1;

-- name: ListFeedbackTemplates :many
select * from feedback_templates order by title, id;

-- name: CreateFeedbackTemplate :one
insert into feedback_templates (title, body, created_by) values ($1, $2, $3)
returning *;

-- name: UpdateFeedbackTemplate :one
update feedback_templates set title = $2, body = $3 where id = $1
returning *;

-- name: DeleteFeedbackTemplate :execrows
delete from feedback_templates where id = $1;
