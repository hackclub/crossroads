-- name: GetUser :one
select * from users where id = $1;

-- name: UpsertUserFromHCA :one
insert into users (
    hca_id, slack_id, email, name, first_name, last_name, birthday, phone_number, address,
    verification_status, ysws_eligible
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
on conflict (hca_id) do update set
    slack_id = excluded.slack_id,
    email = excluded.email,
    name = excluded.name,
    first_name = excluded.first_name,
    last_name = excluded.last_name,
    birthday = excluded.birthday,
    phone_number = excluded.phone_number,
    address = excluded.address,
    verification_status = excluded.verification_status,
    ysws_eligible = excluded.ysws_eligible
returning *;

-- name: SearchUsers :many
-- Matches part of a name or email, or a whole Slack ID. An empty query lists
-- everyone, newest first.
select * from users
where sqlc.arg(query)::text = ''
    or name ilike '%' || sqlc.arg(query)::text || '%'
    or email ilike '%' || sqlc.arg(query)::text || '%'
    or slack_id = sqlc.arg(query)::text
order by id desc
limit 50;

-- name: SetUserRole :one
update users set role = $2 where id = $1
returning *;
