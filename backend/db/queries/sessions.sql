-- name: CreateSession :exec
insert into sessions (token_hash, user_id, expires_at) values ($1, $2, $3);

-- name: GetSessionUser :one
select users.* from sessions
join users on users.id = sessions.user_id
where sessions.token_hash = $1 and sessions.expires_at > now();

-- name: DeleteSession :exec
delete from sessions where token_hash = $1;

-- name: DeleteExpiredSessions :exec
delete from sessions where user_id = $1 and expires_at <= now();
