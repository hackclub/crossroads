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
    -- A name they chose themselves stays; otherwise Hack Club Auth's wins.
    name = case when users.name_customized then users.name else excluded.name end,
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

-- name: SetUserName :one
-- A name someone chose, or (customized false) the one from Hack Club Auth again.
update users set name = $2, name_customized = $3 where id = $1
returning *;

-- name: SetEmporiumAccess :one
-- Lets someone into the Smugglers' Emporium, or (with a null time) out again.
update users set emporium_access_at = $2, emporium_note = $3, emporium_granted_by = $4 where id = $1
returning *;

-- name: SetUserPermissions :one
update users set can_review = $2, can_fraud_review = $3, can_hq_review = $4 where id = $1
returning *;

-- name: GetUserByEmail :one
select * from users where lower(email) = lower(sqlc.arg(email)::text)
order by id
limit 1;

-- name: ListRoleInvites :many
select role_invites.*, users.name as invited_by_name
from role_invites
left join users on users.id = role_invites.invited_by
order by role_invites.created_at desc;

-- name: UpsertRoleInvite :one
-- Inviting an email again changes the role it'll get.
insert into role_invites (email, role, invited_by)
values (lower(sqlc.arg(email)::text), sqlc.arg(role), sqlc.arg(invited_by))
on conflict (email) do update set
    role = excluded.role,
    invited_by = excluded.invited_by,
    created_at = now()
returning *;

-- name: DeleteRoleInvite :execrows
delete from role_invites where id = $1;

-- name: ClaimRoleInvite :one
-- Gives someone signing in the role an admin set for their email, and uses
-- up the invite. No rows if there isn't one.
with invite as (
    delete from role_invites where email = lower(sqlc.arg(email)::text)
    returning role
)
update users set role = invite.role
from invite
where users.id = sqlc.arg(id)
returning users.*;

-- name: SyncSuperadmins :exec
-- Makes the people with these Slack IDs superadmins, and anyone else who was
-- one an admin. main.go has the list.
update users
set role = case when slack_id = any(sqlc.arg(slack_ids)::text[]) then 'superadmin'::user_role else 'admin'::user_role end
where coalesce(slack_id = any(sqlc.arg(slack_ids)::text[]), false) <> (role = 'superadmin');
