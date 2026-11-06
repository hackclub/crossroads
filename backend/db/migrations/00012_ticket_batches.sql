-- +goose Up

-- A ticket can sell in batches: the first batch_size tickets cost price, and
-- each batch after that costs batch_step more than the one before. stock is
-- how many are left across every batch.
alter table shop_items
    add column batch_size integer check (batch_size > 0),
    add column batch_step integer check (batch_step > 0),
    add constraint shop_items_batches check (
        (batch_size is null) = (batch_step is null)
        and (batch_size is null or (kind = 'ticket' and stock is not null))
    ),
    add constraint shop_items_one_price_lock check (kind <> 'price_lock' or max_per_user = 1);

-- On a price lock's order, the ticket price it locked in.
alter table orders add column locked_price integer check (locked_price > 0);

-- +goose Down

alter table orders drop column locked_price;
alter table shop_items
    drop constraint shop_items_one_price_lock,
    drop constraint shop_items_batches,
    drop column batch_step,
    drop column batch_size;
