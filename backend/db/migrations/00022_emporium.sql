-- +goose Up

-- The Smugglers' Emporium: a second shop, hidden from everyone but reviewers,
-- admins, and people an admin lets in (for a project worth rewarding).
alter table shop_items
    add column emporium boolean not null default false,
    -- Grants: each piece is grant_cents, at grant_cents_per_hour of approved
    -- time, so one costs grant_cents / grant_cents_per_hour hours of stamps.
    add column grant_cents integer check (grant_cents > 0),
    add column grant_cents_per_hour integer check (grant_cents_per_hour > 0),
    add constraint shop_items_grant_fields
        check ((kind = 'grant') = (grant_cents is not null and grant_cents_per_hour is not null)),
    add constraint shop_items_grants_in_emporium check (kind <> 'grant' or emporium);

-- A grant order is several pieces at once; everything else is one.
alter table orders
    add column quantity integer not null default 1 check (quantity > 0),
    add column grant_cents integer check (grant_cents > 0); -- what the grant is worth, in total

-- Who an admin let in, when, and why. Reviewers and admins are always in.
alter table users
    add column emporium_access_at timestamptz,
    add column emporium_note text,
    add column emporium_granted_by bigint references users (id);

-- +goose Down
alter table users drop column emporium_granted_by, drop column emporium_note, drop column emporium_access_at;
alter table orders drop column grant_cents, drop column quantity;
delete from shop_items where kind = 'grant' and not exists (select 1 from orders where orders.shop_item_id = shop_items.id);
alter table shop_items
    drop constraint shop_items_grants_in_emporium,
    drop constraint shop_items_grant_fields,
    drop column grant_cents_per_hour, drop column grant_cents, drop column emporium;
