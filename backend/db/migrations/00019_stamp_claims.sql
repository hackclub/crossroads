-- +goose Up

-- A stamp type with a claim code can be claimed at /claim/{code}, once per
-- person, until its supply runs out. It's how a link in an announcement gives
-- out a stamp.
alter table special_stamp_types
    add column claim_code text unique check (claim_code ~ '^[a-z0-9-]+$');

-- Claimed stamps come from neither a box nor an admin.
alter table special_stamps
    add column claimed boolean not null default false,
    drop constraint special_stamps_check,
    add constraint special_stamps_source
        check (num_nonnulls(box_id, granted_by) + claimed::int = 1);

create unique index special_stamps_one_claim_per_type
    on special_stamps (user_id, special_stamp_type_id) where claimed;

-- +goose Down

delete from special_stamps where claimed;
alter table special_stamps
    drop constraint special_stamps_source,
    add constraint special_stamps_check check ((box_id is null) <> (granted_by is null)),
    drop column claimed;
alter table special_stamp_types drop column claim_code;
