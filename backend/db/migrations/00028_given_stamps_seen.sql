-- +goose Up

-- A stamp an admin gives takes over its owner's Home until they've seen it
-- come out, like a project's box until it's opened; seen_at is when they
-- did. Stamps from a box or a claim link come out in front of them, so
-- they're seen as they're made, and so is every stamp from before this.
alter table special_stamps add column seen_at timestamptz default now();
update special_stamps set seen_at = acquired_at;

-- +goose Down

alter table special_stamps drop column seen_at;
