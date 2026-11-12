-- Review counts, for the reviewers' dashboard and for paying reviewers.
--
-- A review is a decision someone made on a ship: its timeline event
-- (ship_events) of kind approval, rejection, fraud_pass, final_approval or
-- sent_back. Comments, notes and edits to a decision aren't reviews. Events
-- are never deleted (ships can't be, and nothing cascades), so a count for a
-- period never changes once the period is over. The event's stage says where
-- the ship was: pending is the first review, fraud_check the fraud check,
-- awaiting_hq HQ sign-off.

-- name: ReviewerStats :many
-- Every person who made a review in [starts, ends), with their counts.
select
    users.id, users.name, users.email, users.slack_id,
    count(*)::bigint as reviews,
    count(*) filter (where d.stage = 'pending')::bigint as first_reviews,
    count(*) filter (where d.stage = 'pending' and d.kind = 'approval')::bigint as first_approvals,
    count(*) filter (where d.stage = 'pending' and d.kind = 'rejection')::bigint as first_rejections,
    count(*) filter (where d.stage = 'fraud_check')::bigint as fraud_checks,
    count(*) filter (where d.stage = 'awaiting_hq')::bigint as hq_decisions,
    -- First reviews whose approval HQ or the fraud check later sent back.
    count(*) filter (where d.stage = 'pending' and exists (
        select 1 from ship_events later
        where later.submission_id = d.submission_id and later.kind = 'sent_back' and later.id > d.id
    ))::bigint as later_sent_back,
    -- The hours the ships claimed, for the first reviews: how much work was looked at.
    coalesce(sum(submissions.new_seconds) filter (where d.stage = 'pending'), 0)::bigint as seconds_first_reviewed,
    max(d.created_at)::timestamptz as last_review_at
from ship_events d
join users on users.id = d.actor_id
join submissions on submissions.id = d.submission_id
where d.kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')
    and d.created_at >= sqlc.arg(starts) and d.created_at < sqlc.arg(ends)
group by users.id
order by reviews desc, users.name, users.id;

-- name: ListReviews :many
-- Every review in [starts, ends), oldest first, optionally by one person:
-- the line items behind ReviewerStats.
select
    d.id, d.created_at, d.kind, d.stage, d.seconds, d.submission_id,
    users.id as reviewer_id, users.name as reviewer_name,
    projects.name as project_name, makers.name as maker_name, submissions.new_seconds,
    (d.stage = 'pending' and exists (
        select 1 from ship_events later
        where later.submission_id = d.submission_id and later.kind = 'sent_back' and later.id > d.id
    ))::boolean as later_sent_back
from ship_events d
join users on users.id = d.actor_id
join submissions on submissions.id = d.submission_id
join projects on projects.id = submissions.project_id
join users makers on makers.id = projects.user_id
where d.kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')
    and d.created_at >= sqlc.arg(starts) and d.created_at < sqlc.arg(ends)
    and (sqlc.narg(reviewer_id)::bigint is null or d.actor_id = sqlc.narg(reviewer_id))
order by d.created_at, d.id;

-- name: ReviewPace :one
-- How the queue is doing: ships in since `since`, hours signed off since
-- then, what's waiting for a first review and for how long, and the median
-- wait for a first review over the last 7 days (-1 when there were none).
select
    (select count(*) from submissions s1 where s1.submitted_at >= sqlc.arg(since))::bigint as shipped,
    (select coalesce(sum(s2.approved_seconds), 0) from submissions s2
        where s2.status = 'approved' and s2.hq_reviewed_at >= sqlc.arg(since))::bigint as seconds_approved,
    (select count(*) from submissions s3 where s3.status = 'pending')::bigint as waiting,
    coalesce((select extract(epoch from now() - min(s4.submitted_at)) from submissions s4 where s4.status = 'pending'), 0)::bigint as oldest_wait_seconds,
    coalesce((
        select percentile_cont(0.5) within group (order by extract(epoch from first.decided_at - s5.submitted_at))
        from (
            select e.submission_id, min(e.created_at) as decided_at from ship_events e
            where e.stage = 'pending' and e.kind in ('approval', 'rejection')
            group by e.submission_id
        ) first
        join submissions s5 on s5.id = first.submission_id
        where first.decided_at >= now() - interval '7 days'
    ), -1)::float8 as median_wait_seconds;
