-- name: AddProjectDM :one
insert into project_dms (user_id, project_id, submission_id, kind, text)
values ($1, $2, $3, $4, $5)
returning *;

-- name: ListProjectDMsToSend :many
-- DMs due a try, oldest first, with who they're for and whether they want
-- them.
select sqlc.embed(project_dms), users.slack_id, users.name, users.dms_enabled
from project_dms
join users on users.id = project_dms.user_id
where project_dms.sent_at is null and project_dms.abandoned_at is null
    and project_dms.next_try_at <= now()
order by project_dms.id
limit 50;

-- name: MarkProjectDMSent :exec
update project_dms
set sent_at = now(), channel_id = $2, message_ts = $3, attempts = attempts + 1, error = null
where id = $1;

-- name: RetryProjectDM :exec
-- A try that failed for a reason that may pass, like Slack being down.
update project_dms set attempts = attempts + 1, error = $2, next_try_at = $3 where id = $1;

-- name: AbandonProjectDM :exec
update project_dms set abandoned_at = now(), error = $2 where id = $1;

-- name: ListProjectDMs :many
-- Every DM about a project, for tests and for looking into what someone was
-- told.
select * from project_dms where project_id = $1 order by id;
