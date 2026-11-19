-- +goose Up

-- Where an item can go, by the country of someone's home address (from Hack
-- Club Auth): everywhere, only to the listed countries, or everywhere except
-- them. Someone whose home country it can't go to doesn't see it and can't
-- buy it. Countries are ISO 3166-1 alpha-2 codes, upper case.
alter table shop_items
    add column region_rule text not null default 'everywhere'
        check (region_rule in ('everywhere', 'only', 'except')),
    add column region_countries text[] not null default '{}',
    add constraint shop_items_region_countries_set
        check ((region_rule = 'everywhere') = (cardinality(region_countries) = 0));

-- +goose Down
alter table shop_items
    drop constraint shop_items_region_countries_set,
    drop column region_countries,
    drop column region_rule;
