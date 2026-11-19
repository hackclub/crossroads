package api

// Where a shop item can go (migration 00034): everywhere, only to some
// countries, or everywhere except some. It's judged by the country of
// someone's home address from Hack Club Auth: an item that can't go there
// isn't listed for them and can't be bought by them. A physical item's
// shipping address is checked too when its country is a code.

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/scherepi/crossroads-backend/db"
)

const (
	regionsEverywhere = "everywhere"
	regionsOnly       = "only"
	regionsExcept     = "except"
)

// regionsJSON is an item's rule: rule is everywhere, only or except, and
// countries are ISO 3166-1 alpha-2 codes (US, IN, DE).
type regionsJSON struct {
	Rule      string   `json:"rule"`
	Countries []string `json:"countries"`
}

func toRegionsJSON(item db.ShopItem) regionsJSON {
	countries := item.RegionCountries
	if countries == nil {
		countries = []string{}
	}
	return regionsJSON{item.RegionRule, countries}
}

// applyRegions sets item's rule from the form, or says what's wrong with it.
func applyRegions(item *db.ShopItem, r regionsJSON) string {
	countries := []string{}
	for _, c := range r.Countries {
		c = strings.ToUpper(strings.TrimSpace(c))
		if !isCountryCode(c) {
			return fmt.Sprintf("%q isn't a country code. Use two letters, like US, IN or DE.", c)
		}
		if !slices.Contains(countries, c) {
			countries = append(countries, c)
		}
	}
	slices.Sort(countries)
	switch {
	case r.Rule != regionsEverywhere && r.Rule != regionsOnly && r.Rule != regionsExcept:
		return "regions.rule has to be everywhere, only or except."
	case r.Rule == regionsEverywhere && len(countries) > 0:
		return "An item that goes everywhere can't list countries."
	case r.Rule != regionsEverywhere && len(countries) == 0:
		return "List at least one country, or make it go everywhere."
	case r.Rule != regionsEverywhere && (item.Kind == db.ShopItemKindTicket || item.Kind == db.ShopItemKindPriceLock):
		return "Rail passes and price locks are for everyone, so they go everywhere."
	}
	item.RegionRule, item.RegionCountries = r.Rule, countries
	return ""
}

func isCountryCode(c string) bool {
	return len(c) == 2 && c[0] >= 'A' && c[0] <= 'Z' && c[1] >= 'A' && c[1] <= 'Z'
}

// goesTo says whether item can go to country, a code; an unknown country
// ("") can't be judged, so everything goes.
func goesTo(item db.ShopItem, country string) bool {
	if country == "" {
		return true
	}
	switch item.RegionRule {
	case regionsOnly:
		return slices.Contains(item.RegionCountries, country)
	case regionsExcept:
		return !slices.Contains(item.RegionCountries, country)
	}
	return true
}

// homeCountry is the country code of someone's home address from Hack Club
// Auth, or "" when there's none or it isn't a code.
func homeCountry(address []byte) string {
	if len(address) == 0 {
		return ""
	}
	var a db.Address
	if json.Unmarshal(address, &a) != nil {
		return ""
	}
	return countryCode(a.Country)
}

// countryCode is c as a code, or "" when it isn't one: addresses typed at
// checkout may say "Brazil" instead of BR, and those aren't checked.
func countryCode(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	if !isCountryCode(c) {
		return ""
	}
	return c
}
