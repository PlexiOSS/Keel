# Keel

Common, non-domain-specific primitives shared across PlexiOSS's Go services (Popplio and friends). Nothing in here should know about bots, servers, staff permissions, or anything else specific to Omniplex that's what keeps it safe to reuse. If a piece of code needs a change that only makes sense for one consumer's business logic, it belongs in that consumer, not here.

## Packages

| Package | What it's for |
|---|---|
| `uapi` | Unified API handling — routing, auth, timeouts, and caching, built in. The backbone of Popplio's public route layer. |
| `doclib` | Documentation types shared by `uapi` routes for generating API docs. |
| `dovewing` | Custom Discord user fetching and caching. |
| `zapchi` | A modified `chi` request logger with [zap](https://github.com/uber-go/zap) support. |
| `crypto` | Common cryptography primitives (random string generation, etc). |
| `pem` | RSA keypair generation, PEM-encoded. |
| `proxy` | A Discord-aware HTTP proxy. |
| `ratelimit` | Rate limiting for HTTP handlers. |
| `hotcache` | A generic cache interface (`HotCache[T]`), with a Redis-backed implementation provided — bring your own if Redis doesn't fit. |
| `genconfig` | Generates config file scaffolding/docs from a Go config struct. |
| `snippets` | Small reusable helpers (zap setup, etc.) that don't warrant their own package. |
| `jsonimpl` | Marshal/unmarshal that transparently uses [sonic](https://github.com/bytedance/sonic) on `amd64`/`linux` and falls back to `encoding/json` everywhere else. |
| `dbutil` | `GetCols(s any) []string` — derives a SQL column list from a struct's `db` tags, so a query and the struct it scans into can't silently drift apart. |
| `taggedunion` | Encodes/decodes Go structs as serde's default externally-tagged enum representation (`{"Variant": {...}}` / `"UnitVariant"`) — for services that need to speak a wire format `encoding/json` has no native equivalent for. |
| `typedresp` | A small typed HTTP response builder (JSON/text/no-content/stream) for services that dispatch to a handler and write its result afterward, rather than writing to `http.ResponseWriter` inline. |
| `ptr` | `Pointer[T](v T) *T`, plus `TruePtr`/`FalsePtr` — taking the address of a literal or non-addressable value inline, for optional request/response fields. |
| `uuidutil` | `Encode(src [16]byte) string` — formats a raw UUID as 8-4-4-4-12 hyphenated hex, without pulling in a UUID library or a database driver's UUID type. |
| `urlutil` | `DifferentHost(candidate, want string) bool` — compares two URLs by host only, treating empty/unparseable input as different. For checking an OAuth `redirect_uri` or a request's `Origin` against a known-good URL. |

`dbutil`, `taggedunion`, and `typedresp` were extracted out of Popplio's `arcadia` package — that code had nothing Arcadia-specific about it, it was just infrastructure that happened to be written there first. If you're touching Popplio and reach for something that feels like "generic plumbing, not business logic," check here before reinventing it — there's now precedent for pulling exactly that kind of code out.

## Using Keel in a consumer (e.g. Popplio)

Popplio depends on a tagged version from `go.mod`:

```
github.com/PlexiOSS/Keel v1.10.0
```

Bumping to a newer Keel release is the usual Go module dance:

```bash
go get github.com/PlexiOSS/Keel@v1.11.0   # or @latest for the newest tag
go mod tidy
```

### Developing against unreleased Keel changes

If you're changing something in Keel and want to test it in Popplio (or another consumer) *before* publishing a new version, point `go.mod` at your local checkout instead of a real version:

```bash
go mod edit -replace github.com/PlexiOSS/Keel=../Keel
go build ./...   # now compiles against your local Keel working copy
```

**Never commit that `replace` line** — it hardcodes a filesystem path that only exists on your machine, and it'll break the build for everyone else the moment they pull it. Drop it before committing:

```bash
go mod edit -dropreplace github.com/PlexiOSS/Keel
go mod tidy
```

### Publishing a new Keel version

Once a change here is ready for consumers to actually pick up:

1. Commit and push it to this repo's `main` branch.
2. Tag the commit with the next semver version (`git tag v1.11.0 && git push origin v1.11.0`) Keel has no other release process, the git tag *is* the release.
3. In each consumer (Popplio, etc.), run `go get github.com/PlexiOSS/Keel@v1.11.0` and commit the resulting `go.mod`/`go.sum` diff.

Bump the minor version for additive, backward-compatible changes (new packages, new functions) and the major version for anything that breaks an existing consumer's build. There's no changelog convention here yet — the commit history and git tags are the record.
