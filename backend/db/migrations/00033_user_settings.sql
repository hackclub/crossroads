-- +goose Up

-- What people choose on the platform's settings page. Tickets, Please DMs
-- them about their projects unless they turn it off. The landing page's
-- trains only carry people who said yes: null is someone who hasn't been
-- asked yet, and the platform asks them.
alter table users
    add column dms_enabled boolean not null default true,
    add column on_landing_page boolean;

-- +goose Down
alter table users
    drop column dms_enabled,
    drop column on_landing_page;
