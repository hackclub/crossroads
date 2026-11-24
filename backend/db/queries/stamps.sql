-- name: AddStampEntry :one
insert into stamp_ledger (user_id, kind, amount, submission_id, order_id, created_by, note)
values ($1, $2, $3, $4, $5, $6, $7)
returning *;

-- name: ListStampEntries :many
select * from stamp_ledger where user_id = $1 order by id desc;

-- name: ForfeitProjectStamps :many
-- A hard reject: takes back what each of the project's ships still holds of
-- the tickets it was paid (see migration 43). Lock the owner first.
insert into stamp_ledger (user_id, kind, amount, submission_id, created_by)
select stamp_ledger.user_id, 'forfeit'::stamp_entry_kind, -sum(stamp_ledger.amount), stamp_ledger.submission_id,
    sqlc.arg(created_by)::bigint
from stamp_ledger
join submissions on submissions.id = stamp_ledger.submission_id
where submissions.project_id = sqlc.arg(project_id)
group by stamp_ledger.user_id, stamp_ledger.submission_id
having sum(stamp_ledger.amount) > 0
returning *;

-- name: RestoreProjectStamps :many
-- Gives back what each of the project's ships has forfeited. Lock the owner
-- first.
insert into stamp_ledger (user_id, kind, amount, submission_id, created_by)
select stamp_ledger.user_id, 'restore'::stamp_entry_kind, -sum(stamp_ledger.amount), stamp_ledger.submission_id,
    sqlc.arg(created_by)::bigint
from stamp_ledger
join submissions on submissions.id = stamp_ledger.submission_id
where submissions.project_id = sqlc.arg(project_id) and stamp_ledger.kind in ('forfeit', 'restore')
group by stamp_ledger.user_id, stamp_ledger.submission_id
having sum(stamp_ledger.amount) < 0
returning *;

-- name: ForfeitedStampsForProject :one
-- What the project's ships have forfeited and not been given back.
select coalesce(-sum(stamp_ledger.amount), 0)::bigint
from stamp_ledger
join submissions on submissions.id = stamp_ledger.submission_id
where submissions.project_id = $1 and stamp_ledger.kind in ('forfeit', 'restore');
