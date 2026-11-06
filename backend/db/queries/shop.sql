-- name: ListShopItems :many
select * from shop_items where active order by position, price, id;

-- name: ListAllShopItems :many
select * from shop_items order by position, price, id;

-- name: GetShopItem :one
select * from shop_items where id = $1;

-- name: LockShopItem :one
select * from shop_items where id = $1 for update;

-- name: CreateShopItem :one
insert into shop_items (kind, name, description, image_url, price, stipend_cents, stock, max_per_user, active, position, batch_size, batch_step)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
returning *;

-- name: UpdateShopItem :one
update shop_items set
    kind = $2, name = $3, description = $4, image_url = $5, price = $6,
    stipend_cents = $7, stock = $8, max_per_user = $9, active = $10, position = $11,
    batch_size = $12, batch_step = $13
where id = $1
returning *;

-- name: TakeShopItemStock :exec
update shop_items set stock = stock - 1 where id = $1 and stock is not null;

-- name: ReturnShopItemStock :exec
update shop_items set stock = stock + 1 where id = $1 and stock is not null;

-- name: CountOrdersOfItem :one
-- How many of an item someone has bought, not counting cancelled orders.
select count(*) from orders
where user_id = $1 and shop_item_id = $2 and status <> 'cancelled';

-- name: CountOrdersByItem :many
select shop_item_id, count(*) from orders
where user_id = $1 and status <> 'cancelled'
group by shop_item_id;

-- name: CreateOrder :one
insert into orders (user_id, shop_item_id, price, shipping_address, status, locked_price)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: CountSold :one
-- How many of an item everyone has bought, not counting cancelled orders.
select count(*) from orders where shop_item_id = $1 and status <> 'cancelled';

-- name: CountSoldByItem :many
select shop_item_id, count(*) from orders
where status <> 'cancelled'
group by shop_item_id;

-- name: GetTicket :one
-- The ticket the shop sells, which price locks apply to: the first active
-- one, if there's more than one.
select * from shop_items where kind = 'ticket' and active order by position, id limit 1;

-- name: LockTicket :one
select * from shop_items where kind = 'ticket' and active order by position, id limit 1 for update;

-- name: HasTicket :one
select exists (
    select 1 from orders join shop_items on shop_items.id = orders.shop_item_id
    where orders.user_id = $1 and shop_items.kind = 'ticket' and orders.status <> 'cancelled'
);

-- name: GetPriceLock :one
-- Someone's price lock, if they have one that wasn't refunded.
select orders.* from orders join shop_items on shop_items.id = orders.shop_item_id
where orders.user_id = $1 and shop_items.kind = 'price_lock' and orders.status <> 'cancelled'
order by orders.id desc
limit 1;

-- name: ListUnusedPriceLocks :many
-- Price locks not refunded, held by people who haven't got a ticket.
select orders.id, orders.user_id from orders join shop_items on shop_items.id = orders.shop_item_id
where shop_items.kind = 'price_lock' and orders.status <> 'cancelled'
    and not exists (
        select 1 from orders as tickets join shop_items as ticket_items on ticket_items.id = tickets.shop_item_id
        where tickets.user_id = orders.user_id and ticket_items.kind = 'ticket' and tickets.status <> 'cancelled'
    )
order by orders.id;

-- name: ListOrders :many
select sqlc.embed(orders), sqlc.embed(shop_items)
from orders
join shop_items on shop_items.id = orders.shop_item_id
where orders.user_id = $1
order by orders.id desc;

-- name: AdminListOrders :many
select sqlc.embed(orders), sqlc.embed(shop_items), sqlc.embed(users)
from orders
join shop_items on shop_items.id = orders.shop_item_id
join users on users.id = orders.user_id
where sqlc.narg(status)::order_status is null or orders.status = sqlc.narg(status)::order_status
order by orders.id;

-- name: LockOrder :one
select * from orders where id = $1 for update;

-- name: SetOrderStatus :one
update orders set status = $2, notes = $3 where id = $1
returning *;

-- name: SetUserVerification :one
update users set verification_status = $2, ysws_eligible = $3 where id = $1
returning *;
