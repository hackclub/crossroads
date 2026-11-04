-- name: ListSubmissionsToSync :many
-- Approved submissions not in Airtable yet, with everything their row needs.
select sqlc.embed(submissions), sqlc.embed(projects), sqlc.embed(users),
    (select count(*) from submissions earlier
        where earlier.project_id = submissions.project_id
            and earlier.status = 'approved'
            and earlier.id < submissions.id) as earlier_approvals
from submissions
join projects on projects.id = submissions.project_id
join users on users.id = projects.user_id
where submissions.status = 'approved' and submissions.airtable_record_id is null
order by submissions.id
limit 100;

-- name: SetSubmissionAirtableRecord :exec
update submissions set airtable_record_id = $2 where id = $1;

-- name: ListOrdersToSync :many
-- Orders that are new or have changed since they were last copied.
select sqlc.embed(orders), sqlc.embed(shop_items), sqlc.embed(users)
from orders
join shop_items on shop_items.id = orders.shop_item_id
join users on users.id = orders.user_id
where orders.airtable_synced_at is null or orders.updated_at > orders.airtable_synced_at
order by orders.id
limit 100;

-- name: MarkOrderSynced :exec
-- synced_at is the updated_at that was copied, so anything that changed
-- during the copy goes next time.
update orders set airtable_record_id = $2, airtable_synced_at = $3 where id = $1;

-- name: FulfillOrderFromAirtable :execrows
update orders set status = 'fulfilled'
where airtable_record_id = $1 and status = 'pending';
