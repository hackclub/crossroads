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
