-- name: ReviewerPayDays :many
-- Each reviewer's reviews by day (midnight to midnight Eastern), newest day
-- first: how many, and how many aren't paid yet. A review is what GET
-- /review/stats counts, except redos: a reviewer changing a review that was
-- sent back to them (migration 39) was already paid for it.
select e.actor_id::bigint as reviewer_id,
    ((e.created_at at time zone 'America/New_York')::date)::text as day,
    count(*) as reviews,
    count(*) filter (where p.ship_event_id is null) as unpaid
from ship_events e
left join review_payment_events p on p.ship_event_id = e.id
where e.kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')
    and not e.redo
    and e.actor_id is not null
group by e.actor_id, day
order by e.actor_id, day desc;

-- name: ListReviewersForPay :many
-- Everyone who has made a review, and what they've been paid for them: in
-- tickets, and in US cents.
select users.id, users.name, users.slack_id,
    coalesce((select sum(rp.amount) from review_payments rp where rp.reviewer_id = users.id and rp.currency = 'tickets'), 0)::bigint as paid_tickets,
    coalesce((select sum(rp.amount) from review_payments rp where rp.reviewer_id = users.id and rp.currency = 'usd'), 0)::bigint as paid_cents
from users
where exists (
    select 1 from ship_events e
    where e.actor_id = users.id and e.kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')
        and not e.redo
)
order by lower(users.name), users.id;

-- name: ListReviewPayments :many
select sqlc.embed(review_payments), reviewer.name as reviewer_name, payer.name as paid_by_name
from review_payments
join users reviewer on reviewer.id = review_payments.reviewer_id
join users payer on payer.id = review_payments.paid_by
where review_payments.currency = any(sqlc.arg(currencies)::text[])
order by review_payments.id desc
limit 100;

-- name: LockUnpaidReviews :many
-- A reviewer's unpaid reviews on some days (YYYY-MM-DD, Eastern), locked so
-- two payments can't take the same ones.
select e.id from ship_events e
where e.actor_id = sqlc.arg(reviewer_id)
    and e.kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')
    and not e.redo
    and ((e.created_at at time zone 'America/New_York')::date)::text = any(sqlc.arg(days)::text[])
    and not exists (select 1 from review_payment_events p where p.ship_event_id = e.id)
order by e.id
for update of e;

-- name: CreateReviewPayment :one
insert into review_payments (reviewer_id, paid_by, currency, reviews, rate, amount, ledger_id, note)
values ($1, $2, $3, $4, $5, $6, $7, $8)
returning *;

-- name: MarkReviewsPaid :execrows
insert into review_payment_events (ship_event_id, payment_id)
select unnest(sqlc.arg(event_ids)::bigint[]), sqlc.arg(payment_id)::bigint;
