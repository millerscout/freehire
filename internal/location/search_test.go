package location

import (
	"reflect"
	"testing"
)

func testSearchIndex() []citySearchEntry {
	tsv := "# header\n" +
		"São Paulo\tbr\tsão paulo|sao paulo\n" +
		"San Diego\tus\tsan diego\n" +
		"San Antonio\tus\tsan antonio\n" +
		"Florianópolis\tbr\tflorianópolis|floripa\n" +
		"Springfield\tus\tspringfield\n" +
		"Springfield\tau\tspringfield\n" // same canonical name, different country — not a collision
	return loadCitySearchIndex(tsv, nil)
}

func TestSearchCitiesNamePrefix(t *testing.T) {
	index := testSearchIndex()

	got := searchCitiesIn(index, "Flor", "", 20)

	want := []CityMatch{{Name: "Florianópolis", Country: "br"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("searchCitiesIn(%q) = %+v, want %+v", "Flor", got, want)
	}
}

func TestSearchCitiesRanksMostPopulousFirst(t *testing.T) {
	index := testSearchIndex()

	got := searchCitiesIn(index, "San", "", 20)

	want := []CityMatch{
		{Name: "San Diego", Country: "us"},
		{Name: "San Antonio", Country: "us"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("searchCitiesIn(%q) = %+v, want %+v (population order preserved)", "San", got, want)
	}
}

func TestSearchCitiesAliasPrefixFallback(t *testing.T) {
	index := testSearchIndex()

	got := searchCitiesIn(index, "floripa", "", 20)

	want := []CityMatch{{Name: "Florianópolis", Country: "br"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("searchCitiesIn(%q) = %+v, want %+v (alias prefix reaches canonical entry)", "floripa", got, want)
	}
}

func TestSearchCitiesDedupesEntryMatchedByNameAndAlias(t *testing.T) {
	index := testSearchIndex()

	// "São Paulo" matches both its own canonical-name prefix and its "sao paulo" alias
	// prefix — it must appear exactly once.
	got := searchCitiesIn(index, "s", "br", 20)

	count := 0
	for _, m := range got {
		if m.Name == "São Paulo" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("São Paulo appeared %d times in results, want exactly 1", count)
	}
}

func TestSearchCitiesCountryFilter(t *testing.T) {
	index := testSearchIndex()

	got := searchCitiesIn(index, "Springfield", "au", 20)

	want := []CityMatch{{Name: "Springfield", Country: "au"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("searchCitiesIn(%q, country=au) = %+v, want %+v", "Springfield", got, want)
	}
}

func TestSearchCitiesCapsResults(t *testing.T) {
	index := testSearchIndex()

	got := searchCitiesIn(index, "s", "", 2)

	if len(got) != 2 {
		t.Errorf("searchCitiesIn with limit=2 returned %d results, want 2", len(got))
	}
}

// TestSearchCitiesOverrideDoesNotHijackAnUnrelatedPlace reproduces a real collision: a
// GeoNames row can legitimately list an incidental alternate name that happens to equal
// another place's override key (e.g. Frankfort, Kentucky lists "Frankfurt" as an
// alternate name, colliding with the "frankfurt" -> Frankfurt-am-Main override). That
// coincidence must not rename or dedupe-away the unrelated row.
func TestSearchCitiesOverrideDoesNotHijackAnUnrelatedPlace(t *testing.T) {
	tsv := "Big City\tde\tbig city|shared\n" + // the genuine override target
		"Small Town\tus\tsmall town|shared\n" // coincidentally shares the "shared" alias
	overrides := map[string]cityEntry{
		"shared": {Name: "Big City Renamed", Country: "de"},
	}
	index := loadCitySearchIndex(tsv, overrides)

	got := searchCitiesIn(index, "Small Town", "", 20)

	want := []CityMatch{{Name: "Small Town", Country: "us"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("searchCitiesIn(%q) = %+v, want %+v (unrelated place must survive, unrenamed)", "Small Town", got, want)
	}
}

func TestSearchCitiesAppliesOverrideDisplayName(t *testing.T) {
	tsv := "Köln\tde\tköln|cologne\n"
	overrides := map[string]cityEntry{
		"köln":    {Name: "Cologne", Country: "de"},
		"cologne": {Name: "Cologne", Country: "de"},
	}
	index := loadCitySearchIndex(tsv, overrides)

	got := searchCitiesIn(index, "colog", "", 20)

	want := []CityMatch{{Name: "Cologne", Country: "de"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("searchCitiesIn(%q) = %+v, want %+v (override wins over raw GeoNames name)", "colog", got, want)
	}
}

func TestSearchCitiesBlankQueryReturnsNothing(t *testing.T) {
	index := testSearchIndex()

	for _, q := range []string{"", "   "} {
		if got := searchCitiesIn(index, q, "", 20); len(got) != 0 {
			t.Errorf("searchCitiesIn(%q) = %+v, want empty", q, got)
		}
	}
}

// TestEmbeddedCitySearchIndex guards the real embedded dataset the same way
// TestEmbeddedCityDict does for cityDict: the index must be built (not empty) and a
// well-known place must be reachable through it, including via its curated override.
func TestEmbeddedCitySearchIndex(t *testing.T) {
	if len(citySearchIndex) < 10000 {
		t.Fatalf("citySearchIndex has %d entries, want tens of thousands", len(citySearchIndex))
	}

	got := SearchCities("Flor", "", citySearchMaxResults)
	found := false
	for _, m := range got {
		if m.Name == "Florianópolis" && m.Country == "br" {
			found = true
		}
	}
	if !found {
		t.Errorf("SearchCities(%q) = %+v, missing Florianópolis", "Flor", got)
	}

	if got := SearchCities("colog", "", citySearchMaxResults); len(got) == 0 || got[0].Name != "Cologne" {
		t.Errorf("SearchCities(%q) = %+v, want Cologne first (curated override)", "colog", got)
	}

	// Regression: an unrelated US place that happens to share an alternate name with an
	// override target must still be searchable by its own name, not hijacked or dropped.
	if got := SearchCities("Frankfort", "us", citySearchMaxResults); len(got) == 0 {
		t.Error(`SearchCities("Frankfort", country=us) = [], want the real Frankfort, US`)
	}
	if got := SearchCities("Campoalegre", "", citySearchMaxResults); len(got) == 0 {
		t.Error(`SearchCities("Campoalegre") = [], want the real Campoalegre, CO`)
	}
}
