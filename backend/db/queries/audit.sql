-- name: AddAuditEvent :exec
-- For requests and sign-ins. Everything else is added by triggers (migration
-- 24). A user_id that isn't anyone's (a request for a user that doesn't
-- exist) is left out rather than losing the row.
insert into audit_log (kind, actor_id, user_id, details)
values (sqlc.arg(kind), sqlc.narg(actor_id), (select id from users where id = sqlc.narg(user_id)::bigint), sqlc.arg(details));

-- name: ListAuditEvents :many
-- The newest events first, older than before_id if it's set. kinds are whole
-- kinds ('stamps.payout') or groups ('stamps'); none means every kind. A
-- user_id finds what someone did and what happened to them.
select audit_log.*, actors.name as actor_name, users.name as user_name
from audit_log
left join users as actors on actors.id = audit_log.actor_id
left join users on users.id = audit_log.user_id
where (sqlc.narg(before_id)::bigint is null or audit_log.id < sqlc.narg(before_id)::bigint)
    and (cardinality(sqlc.arg(kinds)::text[]) = 0
        or audit_log.kind = any(sqlc.arg(kinds)::text[])
        or split_part(audit_log.kind, '.', 1) = any(sqlc.arg(kinds)::text[]))
    and (sqlc.narg(user_id)::bigint is null
        or audit_log.actor_id = sqlc.narg(user_id)::bigint
        or audit_log.user_id = sqlc.narg(user_id)::bigint)
order by audit_log.id desc
limit sqlc.arg(max_rows)::integer;
