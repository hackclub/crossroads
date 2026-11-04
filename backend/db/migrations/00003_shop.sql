-- +goose Up
create type shop_item_kind as enum ('ticket', 'physical', 'stipend');

create table shop_items (
    id bigint generated always as identity primary key,
    kind shop_item_kind not null,
    name text not null,
    description text not null default '',
    image_url text,
    price integer not null check (price > 0), -- in stamps
    stipend_cents integer check (stipend_cents > 0),
    stock integer check (stock >= 0), -- null is unlimited
    max_per_user integer check (max_per_user > 0), -- null is unlimited
    active boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    check ((kind = 'stipend') = (stipend_cents is not null)),
    check (kind <> 'ticket' or max_per_user = 1)
);

create trigger shop_items_set_updated_at before update on shop_items
    for each row execute function set_updated_at();

create type order_status as enum ('pending', 'fulfilled', 'cancelled');

create table orders (
    id bigint generated always as identity primary key,
    user_id bigint not null references users (id),
    shop_item_id bigint not null references shop_items (id),
    price integer not null check (price > 0), -- what was paid, even if the item's price changes later
    status order_status not null default 'pending',
    shipping_address jsonb, -- copied at purchase for physical items
    notes text, -- admins only
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index on orders (user_id, shop_item_id);

create trigger orders_set_updated_at before update on orders
    for each row execute function set_updated_at();

-- +goose Down
drop table orders;
drop type order_status;
drop table shop_items;
drop type shop_item_kind;
