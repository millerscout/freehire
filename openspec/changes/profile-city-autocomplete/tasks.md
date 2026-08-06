## 1. Backend: city search over the dictionary

- [x] 1.1 In `internal/location`, add a population-ordered, (Name, Country)-deduplicated
      `[]cityEntry` built once at init alongside `cityDict` (extend `loadCityDict` or add a
      sibling loader over the same TSV — do not re-parse independently).
- [x] 1.2 Add `SearchCities(query, countryCode string, limit int) []CityMatch` in
      `internal/location`: case-insensitive prefix match on canonical name, falling back to
      alias prefix; optional country filter; population-ranked (source order); blank query
      returns nothing.
- [x] 1.3 Unit tests for `SearchCities`: name-prefix match, alias-prefix match, dedup of an
      entry matched by multiple aliases, country filter, cap enforcement on a broad query,
      blank-query returns empty. (Two rounds of review also surfaced and fixed a
      same-alias-different-place override collision — see search.go's `claimed` guard.)

## 2. Backend: HTTP endpoint

- [ ] 2.1 Add `internal/handler/geo.go`: `geoHandlers` struct + `newGeoHandlers`, following
      the `companiesHandlers`/`CompanySubindustries` pattern (no auth). `SearchCities`
      handler reads `q` and optional `country` query params, calls `location.SearchCities`
      with a limit of 20, responds `{"data": [{"value", "country"}]}` — a raw ISO code, not
      a pre-composed label (the frontend's existing `countryLabel()` composes the display
      string; see design.md's response-shape decision).
- [ ] 2.2 Register `GET /geo/cities` on the `api` group in `internal/handler/handler.go`
      (mirrors the `companiesH` wiring).
- [ ] 2.3 Integration test (`//go:build integration`, per `internal/handler`'s existing
      convention) covering: anonymous request succeeds, `country` narrows results, response
      shape.
- [ ] 2.4 Document `GET /geo/cities` in `web/static/openapi.yaml`.

## 3. Frontend: search helper

- [ ] 3.1 Add `searchCities(query: string, country?: string): Promise<FacetOption[]>` to
      `web/src/lib/facets.ts`, calling `GET /geo/cities`.

## 4. Frontend: wire ProfileForm's city fields

- [ ] 4.1 Replace `baseCity`'s plain `<Input>` with `RemoteSearchSelect`, single-value
      semantics (`include = baseCity ? [baseCity] : []`, `onToggle` replaces/clears),
      search narrowed by `baseCountry`, `fallbackLabel={(v) => v}`.
- [ ] 4.2 Replace the `relocCities` chip-list + Enter-to-add `<Input>` with
      `RemoteSearchSelect` in its native multi-select mode (`include = relocCities`,
      `onToggle` = existing `toggleIn`, `clearOnSelect`), search not narrowed by country.
      Remove the now-dead `cityDraft` state and `addCity()` function.
- [ ] 4.3 Manual verification in the browser: type-to-search on both fields in light and
      dark mode; confirm a pre-existing free-text saved city (not in the dictionary) still
      displays via `fallbackLabel` and survives an unrelated save.

## 5. Verification

- [ ] 5.1 `go build ./... && go vet ./...` and `go vet -tags=integration ./...`.
- [ ] 5.2 `go test ./... ` and `go test -tags=integration ./internal/handler/` (or the
      project's integration-test invocation) green.
