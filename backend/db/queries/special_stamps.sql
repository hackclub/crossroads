-- name: ListSpecialStampTypes :many
select special_stamp_types.*,
    (select count(*) from special_stamps
        where special_stamps.special_stamp_type_id = special_stamp_types.id
            and special_stamps.submission_id is not null) as applied_count
from special_stamp_types
order by id;

-- name: GetSpecialStampType :one
select * from special_stamp_types where id = $1;

-- name: CreateSpecialStampType :one
insert into special_stamp_types (name, description, image_url, rarity, effect, supply_cap)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: UpdateSpecialStampType :one
update special_stamp_types set
    name = $2, description = $3, image_url = $4, rarity = $5, effect = $6, supply_cap = $7
where id = $1
returning *;

-- name: ClaimSpecialStampSupply :one
-- Takes one from a stamp type's supply. Returns no rows if it's run out.
update special_stamp_types set issued_count = issued_count + 1
where id = $1 and (supply_cap is null or issued_count < supply_cap)
returning *;

-- name: CreateSpecialStamp :one
insert into special_stamps (user_id, special_stamp_type_id, box_id, granted_by)
values ($1, $2, $3, $4)
returning *;

-- name: ListSpecialStamps :many
select sqlc.embed(special_stamps), sqlc.embed(special_stamp_types)
from special_stamps
join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
where special_stamps.user_id = $1
order by special_stamps.id desc;

-- name: LockSpecialStamp :one
select sqlc.embed(special_stamps), special_stamp_types.effect
from special_stamps
join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
where special_stamps.id = $1 and special_stamps.user_id = $2
for update of special_stamps;

-- name: ApplySpecialStamp :exec
update special_stamps set submission_id = $2 where id = $1;

-- name: PlaceSpecialStamp :one
-- Sticks a stamp from the box onto a project.
update special_stamps
set project_id = $2, pos_x = $3, pos_y = $4, rotation = $5, placed_at = now()
where id = $1
returning *;

-- name: MoveSpecialStamp :one
-- Moves a stamp around the project it's already on.
update special_stamps set pos_x = $2, pos_y = $3 where id = $1
returning *;

-- name: ListProjectStamps :many
-- The stamps stuck on a project, oldest first so newer ones sit on top.
select sqlc.embed(special_stamps), sqlc.embed(special_stamp_types)
from special_stamps
join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
where special_stamps.project_id = $1
order by special_stamps.placed_at, special_stamps.id;

-- name: SubmissionHasSpecialStamp :one
select exists (select 1 from special_stamps where submission_id = $1);

-- name: WaitingPriorityStamp :one
-- An Express stamp on the project that hasn't given a ship priority yet.
select special_stamps.id
from special_stamps
join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
where special_stamps.project_id = $1
    and special_stamps.submission_id is null
    and special_stamp_types.effect = 'priority_review'
order by special_stamps.placed_at, special_stamps.id
limit 1
for update of special_stamps;

-- name: ProjectHasStampMultiplier :one
select exists (
    select 1 from special_stamps
    join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
    where special_stamps.project_id = $1 and special_stamp_types.effect = 'stamp_multiplier'
);

-- name: PendingSubmissionForProject :one
select * from submissions where project_id = $1 and status = 'pending'
for update;
