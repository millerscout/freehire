## 1. The fetch

- [ ] 1.1 Extend `recallPhrases` in `internal/gmailsync/senders.go` with the wordings
      measured as missed — `received your application`, `interview invite`, `interview
      invitation`, `next steps`, `schedule a call`, `screening` — keeping the existing
      multilingual entries and adding their obvious siblings. Tests assert each new phrase
      appears in the built query and that the existing ones survive.
- [ ] 1.2 Exclude the connected account's own mail from the query (`-from:<address>`), and
      test it: the address is already carried for thread walking, and a top-level fetch
      without it stores both halves of every conversation.
- [ ] 1.3 Test that the query stays a single well-formed clause as the list grows — the
      phrases are OR-ed inside one group and the exclusion sits outside it.

## 2. The listing

- [ ] 2.1 Add the hide predicate and the hidden count to `ListEmails`/`CountEmails` in
      `internal/db/queries/gmail.sql`: omit `status_signal = 'other'` unless asked, never
      omit an unclassified message, and report how many were omitted under the same filters.
      `make sqlc`.
- [ ] 2.2 Carry the option and the count through `inbox.Query` / `inbox.Page`, with unit
      tests for the default (hidden), the opt-in (shown), and the unclassified case.
- [ ] 2.3 Integration test over a real Postgres: a mailbox holding `other`, a signal, and an
      unclassified message lists two by default and three when asked, and reports one hidden.

## 3. HTTP and web

- [ ] 3.1 Accept the opt-in on `GET /me/inbox` and `GET /me/emails`, and return the hidden
      count in the listing's meta.
- [ ] 3.2 Show the count and the control in the inbox. A hidden filter with no indicator is
      the failure this design exists to avoid, so the number is visible without opening
      anything.
- [ ] 3.3 Run `pnpm run check`, `pnpm run lint`, and the design-system ratchets.

## 4. Documentation and gates

- [ ] 4.1 Update `docs/agents/mail-stack.md`: what the fetch is scoped to now, that the
      classifier is the inbox filter and why no blocklist exists, and that the filter is at
      display because the watermark makes a fetch-time filter permanent.
- [ ] 4.2 Run `go build ./...`, `go vet ./...`, `go test ./...`, and
      `go vet -tags=integration ./...` before pushing.
