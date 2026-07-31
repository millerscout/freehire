## 1. Widen the collections registry contract

- [ ] 1.1 Add `Kind` (`KindEditorial`, `KindCredential`) to `Collection` in `internal/collections`, defaulting every existing entry to editorial; test that the registry exposes a kind for each entry
- [ ] 1.2 Introduce `Record{Name string; Meta map[string]string}` and change `Dataset.Parse` to `[]byte → ([]Record, error)`; adapt the eight existing parsers (`ParseYC`, `ParseTechstarsCSV`, `ParseEUStartups`, `ParseCompanyCSV`, `ParseSlugList`, and the rest) to return name-only records, keeping their existing tests green untouched
- [ ] 1.3 Add optional `Gate func(Company, Record) bool` to `Collection`; test that `Gate == nil` tags every name-matched company exactly as before, and that a gate returning false leaves the company untagged
- [ ] 1.4 Add optional `Dataset.ResolveURL func(context.Context, *http.Client) (string, error)`; test that exactly one of `URL`, `Data`, `ResolveURL` may be set and that a resolver's result is what gets fetched

## 2. Company-name normalization for registers

- [ ] 2.1 Add legal-suffix stripping to `internal/collections` (whole word, end of string only: `Ltd`, `Limited`, `PLC`, `LLP`, `CIC`, `B.V.`, `N.V.` and their punctuation variants), applied before `normalize.Slug`
- [ ] 2.2 Test that `ACME ROBOTICS LIMITED` normalizes to `acme-robotics`, that `LIMITED BRANDS INC` keeps its leading `Limited`, and that a name that is only a suffix is not reduced to empty

## 3. Matching guards

- [ ] 3.1 Implement the geography gate: the register's country present in the company's `countries` satisfies a name of two or more tokens
- [ ] 3.2 Implement the single-token rule: a one-token normalized name additionally requires `hq_country` to equal the register's country; test that a company with UK jobs but a non-UK headquarters is rejected and one headquartered in the UK is accepted
- [ ] 3.3 Implement the ambiguity guard: precompute normalized-name counts per dataset and grant nothing for a name occurring more than once; test that both candidates are left untagged
- [ ] 3.4 Implement the UK route gate over `Record.Meta`: tag only when at least one row carries Skilled Worker, any Global Business Mobility route, or Scale-up; test that a Temporary-Worker-only company is rejected while its rows are still parsed

## 4. Register sources

- [ ] 4.1 Implement the GOV.UK URL resolver: Content API → `details.attachments` → select the CSV by title, with an HTML-scrape fallback; test the API path, the fallback on a non-200, the fallback on unparseable JSON, and an error when neither yields a URL
- [ ] 4.2 Implement `ParseUKSponsors` over the register CSV (Organisation Name, Town/City, County, Type & Rating, Route) into records carrying route, town, and rating; test against a committed fixture
- [ ] 4.3 Implement `ParseNLSponsors` over the IND register HTML table (`<th scope="row">` organisation, `<td>` KvK), decoding HTML entities and retaining the KvK as metadata; test against a committed fixture
- [ ] 4.4 Test that a zero-record parse from either source returns an error rather than an empty slice
- [ ] 4.5 Add the `uk-skilled-worker-sponsor` and `nl-recognised-sponsor` registry entries with their titles, descriptions, `KindCredential`, datasets, and gates

## 5. Import worker

- [ ] 5.1 Widen `ListCompanyCollections` in `internal/db/queries/companies.sql` to `slug, collections, countries, hq_country` and run `make sqlc`
- [ ] 5.2 Rework `cmd/import-collections` for record-shaped resolve, per-dataset ambiguity precomputation, and gate application at match time
- [ ] 5.3 Make an empty parse an abort-before-write failure alongside the existing fetch-failure abort; test that no membership is written in either case
- [ ] 5.4 Add `-dry-run` reporting matched / gated-out / unmatched per collection and writing nothing; test that no write path is reached

## 6. Generated frontend contract

- [ ] 6.1 Emit the collection registry (slug, title, description, kind) from `cmd/gen-contracts` into `web/src/lib/generated/contracts.ts`; run `make gen-contracts` and commit the output
- [ ] 6.2 Point `web/src/lib/facets.ts` and the `/collections` hub at the generated registry, delete the hand-kept `COLLECTIONS` from `web/src/lib/collections.ts`, and leave `FILTER_COLLECTIONS` where it is

## 7. UI

- [ ] 7.1 Verify whether the filter machinery tolerates two facet specs on the `collections` param; if not, render one spec whose options carry group headings — record which way it went
- [ ] 7.2 Split the collection facet options by `kind` into "Collections" and "Credentials" groups in the FilterModal
- [ ] 7.3 Add the credential badge to the job card and the company page, naming the issuing register and carrying the disclaimer that the licence belongs to the employer and is not a promise of sponsorship for that role
- [ ] 7.4 Verify the `/collections/uk-skilled-worker-sponsor` landing page renders server-side with its canonical and breadcrumb JSON-LD, as the existing landing-page machinery should give for free

## 8. Verify and ship

- [ ] 8.1 `go build ./... && go vet ./... && go test ./...` green, and the web build and lint at their baseline
- [ ] 8.2 Run `-dry-run` against production; inspect `hq_country` coverage and the per-collection counts, and record the numbers in the change before any write
- [ ] 8.3 Run the real import, then `make reindex` (never stacked with `reindex-companies`), and confirm the facet returns jobs for both credentials
