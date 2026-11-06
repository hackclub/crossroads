-- +goose Up

-- The Golden Spike: every approved ship of the project it's stuck on pays
-- 10% more stamps, as a separate 'bonus' entry in the ledger.
alter type special_stamp_effect add value if not exists 'stamp_multiplier';
alter type stamp_entry_kind add value if not exists 'bonus';

-- Special stamps get stuck onto a project from the owner's stamp box. Once
-- placed they stay on that project for good: the owner can move them around
-- on it (pos_x and pos_y are 0-1 across the project's stamp board, rotation
-- is in degrees) but can't take them off or move them to another project.
-- If a never-shipped project is deleted, its stamps go back to the box.
alter table special_stamps
    add column project_id bigint references projects (id) on delete set null,
    add column pos_x real check (pos_x between 0 and 1),
    add column pos_y real check (pos_y between 0 and 1),
    add column rotation real not null default 0 check (rotation between -45 and 45),
    add column placed_at timestamptz,
    add constraint special_stamps_placed check (
        project_id is null or (pos_x is not null and pos_y is not null and placed_at is not null)
    );

create index on special_stamps (project_id);

-- +goose Down

alter table special_stamps
    drop constraint special_stamps_placed,
    drop column placed_at,
    drop column rotation,
    drop column pos_y,
    drop column pos_x,
    drop column project_id;
-- Enum values can't be dropped in Postgres; 'stamp_multiplier' and 'bonus' stay.
