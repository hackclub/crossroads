-- name: AddStampEntry :one
insert into stamp_ledger (user_id, kind, amount, submission_id, order_id, created_by, note)
values ($1, $2, $3, $4, $5, $6, $7)
returning *;

-- name: ListStampEntries :many
select * from stamp_ledger where user_id = $1 order by id desc;
