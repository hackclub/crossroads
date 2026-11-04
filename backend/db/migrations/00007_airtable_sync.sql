-- +goose Up

-- Approved submissions are copied to Airtable once. Orders are copied again
-- whenever they change after airtable_synced_at.
alter table submissions add column airtable_record_id text;
alter table orders
    add column airtable_record_id text,
    add column airtable_synced_at timestamptz;

create index submissions_to_sync on submissions (id)
    where status = 'approved' and airtable_record_id is null;

-- Recording a sync mustn't count as a change, or every order would be copied
-- again forever, so updated_at now only moves when the order itself does.
drop trigger orders_set_updated_at on orders;
create trigger orders_set_updated_at
    before update of user_id, shop_item_id, price, status, shipping_address, notes on orders
    for each row execute function set_updated_at();

-- +goose Down
drop trigger orders_set_updated_at on orders;
create trigger orders_set_updated_at before update on orders
    for each row execute function set_updated_at();
drop index submissions_to_sync;
alter table orders drop column airtable_synced_at, drop column airtable_record_id;
alter table submissions drop column airtable_record_id;
