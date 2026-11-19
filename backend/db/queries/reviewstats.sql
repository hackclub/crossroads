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

-- name: ReviewsPerDay :many
-- Reviews by everyone on each Eastern day in [starts, ends), for the
-- dashboard's chart. Days without any aren't listed.
select
    to_char(d.created_at at time zone 'America/New_York', 'YYYY-MM-DD')::text as day,
    count(*)::bigint as reviews
from ship_events d
where d.kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')
    and d.created_at >= sqlc.arg(starts) and d.created_at < sqlc.arg(ends)
group by 1
order by 1;

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

-- The review page's Review stats tab (api/reviewstats.go, reviewInsights).

-- name: WaitingByStage :many
-- What's waiting at each stage now: how many, the median wait and the
-- oldest. A ship's wait at a stage starts when it got there: its last
-- approval, pass, send-back, reopening or flag, or when it was shipped.
-- Stages with nothing waiting aren't listed.
with waiting as (
    select s.id, s.status, projects.name as project_name,
        greatest(s.submitted_at, coalesce((
            select max(e.created_at) from ship_events e
            where e.submission_id = s.id and e.kind in ('approval', 'fraud_pass', 'sent_back', 'reopened', 'flagged')
        ), s.submitted_at)) as entered_at
    from submissions s
    join projects on projects.id = s.project_id
    where s.status in ('pending', 'fraud_check', 'awaiting_hq')
), oldest as (
    select distinct on (w.status) w.status, w.id, w.project_name, w.entered_at
    from waiting w
    order by w.status, w.entered_at, w.id
)
select
    w.status::text as stage,
    count(*)::bigint as waiting,
    (percentile_cont(0.5) within group (order by extract(epoch from now() - w.entered_at)))::float8 as median_wait_seconds,
    o.id as oldest_id,
    o.project_name as oldest_project,
    extract(epoch from now() - o.entered_at)::bigint as oldest_wait_seconds
from waiting w
join oldest o on o.status = w.status
group by w.status, o.id, o.project_name, o.entered_at
order by w.status;

-- name: DecisionsByStage :one
-- The decisions made in [starts, ends), by stage and kind: for the
-- rejection rates, and the hours HQ signed off.
select
    count(*) filter (where stage = 'pending' and kind = 'approval')::bigint as first_approvals,
    count(*) filter (where stage = 'pending' and kind = 'rejection')::bigint as first_rejections,
    count(*) filter (where stage = 'fraud_check' and kind = 'fraud_pass')::bigint as fraud_passes,
    count(*) filter (where stage = 'fraud_check' and kind = 'rejection')::bigint as fraud_rejections,
    count(*) filter (where stage = 'fraud_check' and kind = 'sent_back')::bigint as fraud_sent_back,
    count(*) filter (where stage = 'awaiting_hq' and kind = 'final_approval')::bigint as hq_sign_offs,
    count(*) filter (where stage = 'awaiting_hq' and kind = 'rejection')::bigint as hq_rejections,
    count(*) filter (where stage = 'awaiting_hq' and kind = 'sent_back')::bigint as hq_sent_back,
    coalesce(sum(seconds) filter (where kind = 'final_approval'), 0)::bigint as seconds_signed_off,
    count(distinct actor_id)::bigint as reviewers
from ship_events
where kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')
    and created_at >= sqlc.arg(starts) and created_at < sqlc.arg(ends);

-- name: ShipFunnel :one
-- Where the ships shipped in [starts, ends) have got to, each counted at
-- every step it ever reached (a ship that was sent back still reached HQ).
with cohort as (
    select s.id, s.status from submissions s
    where s.submitted_at >= sqlc.arg(starts) and s.submitted_at < sqlc.arg(ends)
)
select
    count(*)::bigint as shipped,
    count(*) filter (where exists (
        select 1 from ship_events e where e.submission_id = c.id and e.stage = 'pending' and e.kind in ('approval', 'rejection')
    ))::bigint as first_reviewed,
    count(*) filter (where exists (
        select 1 from ship_events e where e.submission_id = c.id and e.stage = 'pending' and e.kind = 'approval'
    ))::bigint as passed_first_review,
    count(*) filter (where c.status = 'fraud_check' or exists (
        select 1 from ship_events e where e.submission_id = c.id and e.stage = 'fraud_check'
    ))::bigint as fraud_checked,
    count(*) filter (where c.status in ('awaiting_hq', 'approved') or exists (
        select 1 from ship_events e where e.submission_id = c.id and e.stage = 'awaiting_hq'
    ))::bigint as reached_hq,
    count(*) filter (where c.status = 'approved')::bigint as signed_off,
    count(*) filter (where c.status = 'rejected')::bigint as rejected,
    count(*) filter (where c.status in ('pending', 'fraud_check', 'awaiting_hq'))::bigint as in_review
from cohort c;

-- name: ReviewTimes :one
-- Median times, -1 when there were none: shipping to the first review, for
-- ships first reviewed in [starts, ends); and shipping to HQ's sign-off, for
-- ships signed off then.
with firsts as (
    select e.submission_id, min(e.created_at) as decided_at from ship_events e
    where e.stage = 'pending' and e.kind in ('approval', 'rejection')
    group by e.submission_id
), first_waits as (
    select extract(epoch from f.decided_at - s.submitted_at) as wait
    from firsts f
    join submissions s on s.id = f.submission_id
    where f.decided_at >= sqlc.arg(starts)::timestamptz and f.decided_at < sqlc.arg(ends)::timestamptz
), sign_off_waits as (
    select extract(epoch from e.created_at - s.submitted_at) as wait
    from ship_events e
    join submissions s on s.id = e.submission_id
    where e.kind = 'final_approval' and e.created_at >= sqlc.arg(starts)::timestamptz and e.created_at < sqlc.arg(ends)::timestamptz
)
select
    coalesce((select percentile_cont(0.5) within group (order by fw.wait) from first_waits fw), -1)::float8 as median_first_review_seconds,
    coalesce((select percentile_cont(0.5) within group (order by so.wait) from sign_off_waits so), -1)::float8 as median_sign_off_seconds;
