-- name: ListShipsForPrereview :many
-- Ships in review, oldest first: without a pre-review unless include_done.
select submissions.id
from submissions
left join ship_prereviews on ship_prereviews.submission_id = submissions.id
where submissions.status in ('pending', 'fraud_check', 'awaiting_hq')
  and (sqlc.arg(include_done)::boolean or ship_prereviews.submission_id is null)
order by submissions.submitted_at, submissions.id
limit sqlc.arg(max_ships);

-- name: CountShipsWithoutPrereview :one
select count(*)
from submissions
left join ship_prereviews on ship_prereviews.submission_id = submissions.id
where submissions.status in ('pending', 'fraud_check', 'awaiting_hq')
  and ship_prereviews.submission_id is null;

-- name: SavePrereview :exec
insert into ship_prereviews (submission_id, result, source, model, created_by)
values ($1, $2, $3, $4, $5)
on conflict (submission_id) do update set
    result = excluded.result,
    source = excluded.source,
    model = excluded.model,
    created_by = excluded.created_by,
    created_at = now();

-- name: GetPrereview :one
select ship_prereviews.*, coalesce(users.name, '')::text as created_by_name
from ship_prereviews
left join users on users.id = ship_prereviews.created_by
where submission_id = $1;

-- name: GetSubmissionStatus :one
select status from submissions where id = $1;
