-- name: ListSubmissionsToSync :many
-- Approved submissions not in Airtable yet, with everything their row needs.
select sqlc.embed(submissions), sqlc.embed(projects), sqlc.embed(users),
    (select count(*) from submissions earlier
        where earlier.project_id = submissions.project_id
            and earlier.status = 'approved'
            and earlier.id < submissions.id) as earlier_approvals,
    coalesce(reviewer.name, '')::text as reviewer_name,
    coalesce(fraud_reviewer.name, '')::text as fraud_reviewer_name,
    coalesce(hq_reviewer.name, '')::text as hq_reviewer_name,
    -- What the reviewer approved, before any fraud cut or HQ change.
    (select ship_events.seconds from ship_events
        where ship_events.submission_id = submissions.id and ship_events.kind = 'approval'
        order by ship_events.id desc limit 1) as reviewer_seconds
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
left join users reviewer on reviewer.id = submissions.reviewer_id
left join users fraud_reviewer on fraud_reviewer.id = submissions.fraud_reviewer_id
left join users hq_reviewer on hq_reviewer.id = submissions.hq_reviewer_id
where submissions.status = 'approved' and submissions.airtable_record_id is null
order by submissions.id
limit 100;

-- name: ListSubmissionsToSubmitUnified :many
-- Rows already in Airtable, signed off by HQ, but not sent to the Unified
-- YSWS DB yet: their owner wasn't YSWS-eligible, or something the Unified YSWS
-- DB needs was missing (see unified.Missing, checked in Go). The columns are
-- ListSubmissionsToSync's, so the row can be sent again in full. Ships
-- approved before HQ sign-off existed (no hq_reviewer_id) are left for a
-- person to tick, as before.
select sqlc.embed(submissions), sqlc.embed(projects), sqlc.embed(users),
    (select count(*) from submissions earlier
        where earlier.project_id = submissions.project_id
            and earlier.status = 'approved'
            and earlier.id < submissions.id) as earlier_approvals,
    coalesce(reviewer.name, '')::text as reviewer_name,
    coalesce(fraud_reviewer.name, '')::text as fraud_reviewer_name,
    coalesce(hq_reviewer.name, '')::text as hq_reviewer_name,
    -- What the reviewer approved, before any fraud cut or HQ change.
    (select ship_events.seconds from ship_events
        where ship_events.submission_id = submissions.id and ship_events.kind = 'approval'
        order by ship_events.id desc limit 1) as reviewer_seconds
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
left join users reviewer on reviewer.id = submissions.reviewer_id
left join users fraud_reviewer on fraud_reviewer.id = submissions.fraud_reviewer_id
left join users hq_reviewer on hq_reviewer.id = submissions.hq_reviewer_id
where submissions.status = 'approved'
    and submissions.airtable_record_id is not null
    and submissions.unified_submitted_at is null
    and submissions.hq_reviewer_id is not null
    and users.ysws_eligible
order by submissions.id;

-- name: MarkUnifiedSubmitted :exec
update submissions set unified_submitted_at = now() where id = $1;

-- name: SetSubmissionAirtableRecord :exec
update submissions set airtable_record_id = $2 where id = $1;

-- name: ListOrdersToSync :many
-- Orders that are new or have changed since they were last copied. Price
-- locks stay out: there's nothing to send, and the ticket order says what
-- was paid.
select sqlc.embed(orders), sqlc.embed(shop_items), sqlc.embed(users)
from orders
join shop_items on shop_items.id = orders.shop_item_id
join users on users.id = orders.user_id
where (orders.airtable_synced_at is null or orders.updated_at > orders.airtable_synced_at)
    and shop_items.kind <> 'price_lock'
order by orders.id
limit 100;

-- name: MarkOrderSynced :exec
-- synced_at is the updated_at that was copied, so anything that changed
-- during the copy goes next time.
update orders set airtable_record_id = $2, airtable_synced_at = $3 where id = $1;

-- name: FulfillOrderFromAirtable :execrows
update orders set status = 'fulfilled'
where airtable_record_id = $1 and status = 'pending';
