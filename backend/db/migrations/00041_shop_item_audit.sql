-- +goose Up

-- The audit log (migration 24) gets shop_item.changed whenever an admin
-- changes what an item costs, where it can go, whether it's sold, or what it
-- is: each changed field's old and new value, so a price change can be read
-- back without the request before it. Stock isn't watched, since every
-- purchase changes it (orders are in the log already). Like role.changed,
-- who did it isn't on the row; the request row at the same moment says.
-- +goose StatementBegin
create function audit_shop_item_change() returns trigger language plpgsql as $$
declare
    changes jsonb := '{}';
begin
    if old.price is distinct from new.price then
        changes := changes || jsonb_build_object('price', jsonb_build_object('from', old.price, 'to', new.price));
    end if;
    if old.active is distinct from new.active then
        changes := changes || jsonb_build_object('active', jsonb_build_object('from', old.active, 'to', new.active));
    end if;
    if old.region_rule is distinct from new.region_rule or old.region_countries is distinct from new.region_countries then
        changes := changes || jsonb_build_object('regions', jsonb_build_object(
            'from', jsonb_build_object('rule', old.region_rule, 'countries', to_jsonb(old.region_countries)),
            'to', jsonb_build_object('rule', new.region_rule, 'countries', to_jsonb(new.region_countries))));
    end if;
    if old.name is distinct from new.name then
        changes := changes || jsonb_build_object('name', jsonb_build_object('from', old.name, 'to', new.name));
    end if;
    if old.emporium is distinct from new.emporium then
        changes := changes || jsonb_build_object('emporium', jsonb_build_object('from', old.emporium, 'to', new.emporium));
    end if;
    if old.stipend_cents is distinct from new.stipend_cents then
        changes := changes || jsonb_build_object('stipend_cents', jsonb_build_object('from', old.stipend_cents, 'to', new.stipend_cents));
    end if;
    if old.grant_cents is distinct from new.grant_cents then
        changes := changes || jsonb_build_object('grant_cents', jsonb_build_object('from', old.grant_cents, 'to', new.grant_cents));
    end if;
    if old.grant_cents_per_hour is distinct from new.grant_cents_per_hour then
        changes := changes || jsonb_build_object('grant_cents_per_hour', jsonb_build_object('from', old.grant_cents_per_hour, 'to', new.grant_cents_per_hour));
    end if;
    if old.max_per_user is distinct from new.max_per_user then
        changes := changes || jsonb_build_object('max_per_user', jsonb_build_object('from', old.max_per_user, 'to', new.max_per_user));
    end if;
    if changes <> '{}' then
        insert into audit_log (kind, details) values (
            'shop_item.changed', jsonb_build_object('item_id', new.id, 'item', new.name, 'changes', changes));
    end if;
    return null;
end;
$$;
-- +goose StatementEnd

create trigger shop_items_audit_change after update on shop_items
    for each row execute function audit_shop_item_change();

-- +goose Down
drop trigger shop_items_audit_change on shop_items;
drop function audit_shop_item_change();
