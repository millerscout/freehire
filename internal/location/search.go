package location

import (
	"bufio"
	"strings"
)

// citySearchMaxResults bounds a single SearchCities call, matching the cap the other
// remote-search facets (company, subindustry) already apply to their result lists.
const citySearchMaxResults = 20

// CityMatch is one city-search result: a resolved place's canonical display data.
type CityMatch struct {
	Name    string
	Country string
}

// citySearchEntry is one deduplicated place in population order, carrying its own
// lowercase aliases so a search can fall back to alias-prefix matching. Distinct from
// cityDict (an alias -> entry map with no ordering) because search needs both order and,
// for a given place, every alias that reaches it.
type citySearchEntry struct {
	cityEntry
	aliases []string
}

// citySearchIndex is built once at init from the same embedded TSV cityDict uses, with
// the same curated overrides layered on top, so its display names match what cityDict
// itself resolves for the same place.
var citySearchIndex = loadCitySearchIndex(citiesTSV, cityOverrides)

// loadCitySearchIndex walks the TSV in file order (population-sorted, same as
// loadCityDict), resolving each row independently of every other row so two places that
// happen to share a plain canonical name — e.g. two "Springfield"s in different countries
// — each keep their own entry rather than colliding, unlike an alias-keyed lookup.
//
// An override renames a row only when the override's country matches the row's own raw
// country. A GeoNames alternate-name list can legitimately list an incidental name shared
// with an unrelated place in another country (Frankfort, Kentucky lists "Frankfurt" as an
// alternate name, the same string the "frankfurt" -> Frankfurt-am-Main override keys on);
// without the country guard, that coincidence would rename Frankfort to "Frankfurt" and
// then dedupe it away entirely as a false duplicate of the German city. Every current
// override's target country matches the place it curates, so the guard costs nothing for
// the cases it is meant to fire on.
//
// Rows that resolve to the same final (Name, Country) collapse to the first (most
// populous) occurrence, keeping that occurrence's own alias list for the alias-prefix
// fallback.
func loadCitySearchIndex(tsv string, overrides map[string]cityEntry) []citySearchEntry {
	seen := map[cityEntry]bool{}
	var out []citySearchEntry
	sc := bufio.NewScanner(strings.NewReader(tsv))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSuffix(sc.Text(), "\r")
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		aliases := strings.Split(parts[2], "|")
		final := cityEntry{Name: parts[0], Country: parts[1]}
		for _, alias := range aliases {
			if ov, ok := overrides[alias]; ok && ov.Country == final.Country {
				final = ov
				break
			}
		}
		if seen[final] {
			continue
		}
		seen[final] = true
		out = append(out, citySearchEntry{cityEntry: final, aliases: aliases})
	}
	return out
}

// SearchCities returns the cities whose canonical name or a known alias starts with
// query (case-insensitively), most-populous first, optionally restricted to one ISO
// 3166-1 alpha-2 country. A blank query returns nothing.
func SearchCities(query, countryCode string, limit int) []CityMatch {
	return searchCitiesIn(citySearchIndex, query, countryCode, limit)
}

// searchCitiesIn is SearchCities' pure matching logic over an injected index, so tests
// can exercise it against a small fixture instead of the embedded 34k-row dataset.
func searchCitiesIn(index []citySearchEntry, query, countryCode string, limit int) []CityMatch {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	country := strings.ToLower(countryCode)

	out := make([]CityMatch, 0, limit)
	matched := map[cityEntry]bool{}
	add := func(e citySearchEntry) {
		out = append(out, CityMatch{Name: e.Name, Country: e.Country})
		matched[e.cityEntry] = true
	}

	// Pass 1: canonical-name prefix, population order.
	for _, e := range index {
		if len(out) >= limit {
			return out
		}
		if country != "" && e.Country != country {
			continue
		}
		if strings.HasPrefix(strings.ToLower(e.Name), q) {
			add(e)
		}
	}

	// Pass 2: alias prefix fallback, for places pass 1 didn't already surface.
	for _, e := range index {
		if len(out) >= limit {
			return out
		}
		if (country != "" && e.Country != country) || matched[e.cityEntry] {
			continue
		}
		for _, alias := range e.aliases {
			if strings.HasPrefix(alias, q) {
				add(e)
				break
			}
		}
	}

	return out
}
