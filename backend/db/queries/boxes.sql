-- name: ListBoxes :many
select boxes.*, special_stamps.id as special_stamp_id
from boxes
left join special_stamps on special_stamps.box_id = boxes.id
where boxes.user_id = $1
order by boxes.id desc;

-- name: LockBox :one
select * from boxes where id = $1 and user_id = $2 for update;

-- name: MarkBoxOpened :exec
update boxes set opened_at = now() where id = $1;

-- name: GrantBox :one
insert into boxes (user_id, box_type_id, granted_by) values ($1, $2, $3)
returning *;

-- name: ListBoxTypes :many
select * from box_types order by id;

-- name: GetBoxType :one
select * from box_types where id = $1;

-- name: CreateBoxType :one
insert into box_types (name, description, image_url) values ($1, $2, $3)
returning *;

-- name: UpdateBoxType :one
update box_types set name = $2, description = $3, image_url = $4 where id = $1
returning *;

-- name: ListBoxDrops :many
select * from box_drops order by box_type_id, special_stamp_type_id;

-- name: ListAvailableDrops :many
-- Every box type's drops whose stamp type still has supply left.
select box_drops.box_type_id, box_drops.weight, sqlc.embed(special_stamp_types)
from box_drops
join special_stamp_types on special_stamp_types.id = box_drops.special_stamp_type_id
where special_stamp_types.supply_cap is null or special_stamp_types.issued_count < special_stamp_types.supply_cap
order by box_drops.box_type_id, special_stamp_types.id;

-- name: DeleteBoxDrops :exec
delete from box_drops where box_type_id = $1;

-- name: AddBoxDrop :exec
insert into box_drops (box_type_id, special_stamp_type_id, weight) values ($1, $2, $3);

-- name: BoxTypesOnlyUncappedBy :many
-- Box types where this stamp type is the only drop without a supply cap.
select box_drops.box_type_id from box_drops
where box_drops.special_stamp_type_id = $1
    and not exists (
        select 1 from box_drops other
        join special_stamp_types on special_stamp_types.id = other.special_stamp_type_id
        where other.box_type_id = box_drops.box_type_id
            and other.special_stamp_type_id <> box_drops.special_stamp_type_id
            and special_stamp_types.supply_cap is null
    );

-- name: ListMilestones :many
select * from milestones order by approved_seconds;

-- name: CreateMilestone :one
insert into milestones (name, approved_seconds, box_type_id) values ($1, $2, $3)
returning *;

-- name: GrantMilestoneToQualifiedUsers :many
-- Grants a new milestone's box to everyone who's already past it.
insert into boxes (user_id, box_type_id, milestone_id)
select projects.user_id, sqlc.arg(box_type_id)::bigint, sqlc.arg(milestone_id)::bigint
from submissions
join projects on projects.id = submissions.project_id
where submissions.status = 'approved'
group by projects.user_id
having sum(submissions.approved_seconds) >= sqlc.arg(approved_seconds)::integer
on conflict (user_id, milestone_id) do nothing
returning *;
