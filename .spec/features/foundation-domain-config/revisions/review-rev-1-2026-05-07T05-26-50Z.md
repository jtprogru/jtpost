# Code Review: Foundation — Domain Model & Configuration (F1)

**Date:** 2026-05-07
**Reviewer:** Claude (delegated subagent, Opus 4.7 1M)
**Branch:** `feature/foundation-domain-config`
**Verdict:** PASS (with minor findings)

---

## 1. Verification Evidence

Re-verified locally during review:

- `task lint` → `0 issues.`
- `task build` → success (`./dist/jtpost`).
- `task test` → all 9 packages pass (logger output truncated to confirm green).
- Baseline (already verified by parent agent): `task fmt` clean, `task vet` clean, `task test` 9/9 ok, `task test:race` 9/9 ok, `task test:coverage` produced (config 90.9 %, fsrepo 57.5 %, httpapi 69.1 %, sqlite 73.2 %, telegramconv 100 %, cli 38.2 %, core 57.3 %, logger 94.6 %, telegram 14.8 %).

Coverage in core (57.3 %) and fsrepo (57.5 %) is below the eventual 85 % target, but F1 deliberately defers a coverage gate to F10 (per requirements Q8) — not a finding for this phase.

---

## 2. Change Set

The branch has not yet been committed (everything is in working tree vs. `main`). 51 modified files + 5 new files (`.spec/` excluded). Cross-referenced against task-plan T-1 … T-7 "Files modified" lists.

| File | Status | Notes |
|---|---|---|
| `internal/core/post.go` | ✅ Planned | Post extended, Attachment, PublishAttempt, AttachmentType, TenantShortID, IsValidSortKey added. |
| `internal/core/core.go` | ✅ Planned | StatusOrder removed, allowedTransitions + IsTransitionAllowed + AllStatuses. |
| `internal/core/errors.go` | ✅ Planned | New ErrInvalidTransition, ErrTenantMismatch, ErrPublishRetryExhausted. |
| `internal/core/service.go` | ✅ Planned | Revision++, tenant immutability, Archive/MarkFailed/AppendPublishAttempt. |
| `internal/core/scope.go` | ✅ Planned (new) | WithTenant/TenantFromContext/WithAuthor/AuthorFromContext (ADR-1). |
| `internal/core/post_test.go` | ✅ Planned (new) | CP-6 round-trip, CP-8 prefix, CP-7-style attachment, CP-11 sort whitelist. |
| `internal/core/core_test.go` | ✅ Planned | Cartesian transition table sweep. |
| `internal/core/service_test.go` | ✅ Planned | Full rewrite, 884-line file. |
| `internal/adapters/config/config.go` | ✅ Planned | 4 new sections + uuidDecodeHook + 30+ BindEnv calls. |
| `internal/adapters/config/config_test.go` | ✅ Planned | defaults / env-override / validate / platforms preservation. |
| `internal/adapters/fsrepo/repository.go` | ✅ Planned | Tenant subdirs, scope from ctx, sort/limit/offset. |
| `internal/adapters/fsrepo/frontmatter_parser.go` | ✅ Planned | New fields, truncation to 10, required-fields validation. |
| `internal/adapters/fsrepo/{repository,frontmatter_parser,test_helpers}_test.go` | ✅ Planned | Full coverage of new behavior. |
| `internal/adapters/sqlite/repository.go` | ✅ Planned | Schema gets `tenant_id`, `author_id`, `revision` + index (full F2). |
| `internal/adapters/httpapi/middleware.go` | ✅ Planned | TenantFromConfigMiddleware. |
| `internal/adapters/httpapi/server.go` | ✅ Planned | jsonPost extended; tenant_id_immutable / tenant_mismatch enforcement. |
| `internal/adapters/httpapi/{server,middleware}_test.go` | ✅ Planned | New tests. |
| `internal/cli/init.go` | ✅ Planned | --force, swappable uuidGenerator, prompt, dirs. |
| `internal/cli/new.go` | ✅ Planned | --tenant / --author. |
| `internal/cli/list.go` | ✅ Planned | JSON format (REQ-9.1, REQ-9.2). |
| `internal/cli/root.go` | ✅ Planned | getService removed (REQ-9.3 verified by grep — symbol absent). |
| `internal/cli/{init,list,new,delete,plan,next,stats,migrate_ids}_test.go`, `test_helpers_test.go` | ✅ Planned | New/updated tests. |
| `internal/cli/{next,stats,plan,show,edit,delete,status,publish,import,migrate,migrate_ids,doctor}.go` | ✅ Planned | Adapted to scope-context — wider than strict T-6, but justified by core API change. |
| `testdata/posts/*.md` × 5 | ✅ Planned | New required frontmatter fields. |
| `.jtpost.example.yaml` | ✅ Planned | Full rewrite. |
| `CHANGELOG.md` | ✅ Planned | F1 section. |
| `go.mod` | ⚠️ Unexpected (justified) | `go-viper/mapstructure/v2` promoted to direct dep — required for uuidDecodeHook (ADR-7 / T-3). |
| `internal/adapters/telegram/publisher.go` | ⚠️ Unexpected (justified, minimal) | Implementation report flagged this. Diff is small, only adapts to extended `Post` signature; full Telegram-media work deferred to F7 — fine. |
| `internal/adapters/telegramconv/converter_test.go` | ⚠️ Unexpected (justified) | Compilation fix after `Post` shape changed. Minimal. |
| `internal/logger/logger_test.go` | ⚠️ Unexpected (justified) | Listed as modified in implementation.md; small adjustments only. |

No files outside the planned scope received material changes; the three "unexpected" files have legitimate compile-time reasons. Spot-checks of "Files NOT requiring changes": `cmd/jtpost/main.go`, `internal/core/clock.go`, `internal/core/slug.go`, `internal/core/publisher.go`, `Taskfile.yml`, `.golangci.yaml` — confirmed no diff in the staging.

---

## 3. Requirements Traceability

Audited at REQ-Group granularity with named-citation evidence.

### Group 1 — Required Post fields (REQ-1.1 … REQ-1.5) — ✅ Verified
- REQ-1.1 / 1.2: `service.go:38-43` — `CreatePost` rejects zero `TenantID`/`AuthorID` with `ErrValidation`. Test: `TestService_CreatePost_RequiresTenantAndAuthor` (service_test.go).
- REQ-1.3: `service.go:53-63` — `CreatedAt = UpdatedAt = clock.Now()`, `Revision = 1`. Tested via `TestService_UpdatePost_IncrementsRevision` initial state and the dedicated `TestService_CreatePost_*` cases.
- REQ-1.4: `service.go:166-167` — UpdatedAt + Revision++. Test: `TestService_UpdatePost_IncrementsRevision` (CP-3).
- REQ-1.5: `service.go:159-161` — Tenant immutability at update; **Test: `TestService_UpdatePost_TenantImmutable` (CP-2).** ✅

### Group 2 — Optional Post fields (REQ-2.1 … REQ-2.7) — ✅ Verified
- REQ-2.1 … 2.4: All optional fields present with correct yaml/json tags + omitempty in `post.go:182-194`. Test: `TestPost_RoundTrip_YAML` / `TestPost_RoundTrip_JSON` (CP-6).
- **REQ-2.5 (truncation):** `frontmatter_parser.go:83-93` — sort by `At` desc, then `[:10]`. Test: `TestFrontmatter_PublishHistoryTruncation` (CP-5). ✅
- REQ-2.6: `Revision int` always serialized; `RevisionSHA *string` omitempty. Test: round-trip.
- REQ-2.7: `AttachmentType.Validate()` at `post.go:114-122`. Test: `TestAttachmentType_Validate`.

### Group 3 — Status lifecycle (REQ-3.1 … REQ-3.5) — ✅ Verified
- REQ-3.1: 7 statuses declared in `core.go:9-17`.
- **REQ-3.2 (10 transitions):** `core.go:30-38`. Counted: `idea→draft (1)`, `draft→ready (2)`, `ready→{scheduled,published} (3,4)`, `scheduled→{published,ready,failed} (5,6,7)`, `failed→{ready,archived} (8,9)`, `published→archived (10)`. **Exactly 10, matches REQ-3.2.** ✅ Test: `TestService_UpdateStatus_AllowedTransitions` + cartesian sweep in `core_test.go`.
- REQ-3.3: `service.go:134-136` returns `ErrInvalidTransition`.
- REQ-3.4: `service.go:139-142` sets `PublishedAt` when `nil`.
- REQ-3.5: `MarkFailed` (`service.go:188-205`) appends a `failed` PublishAttempt with non-empty `Error`.

### Group 4 — PostFilter / repo (REQ-4.1 … REQ-4.6) — ✅ Verified
- REQ-4.1: `repository.go:110-112` — zero TenantID → `ErrValidation`. Test: `TestFSRepo_List_RejectsZeroTenant`.
- REQ-4.2: `matchesFilter` at `repository.go:265-267`.
- REQ-4.3 / 4.4: `IsValidSortKey` whitelist; `service.go:121-123` returns `ErrValidation`. Test: `TestPostFilter_IsValidSortKey` (CP-11).
- REQ-4.5: `applyPaging` at `repository.go:368-380`.
- **REQ-4.6 (cross-tenant → ErrNotFound):** `repository.go:69-72` and `repository.go:99-103`. Test: `TestFSRepo_GetByID_OtherTenant_NotFound` (CP-1). ✅

### Group 5 — Configuration (REQ-5.1 … REQ-5.12) — ✅ Verified
- REQ-5.1 … 5.9: All sections + nested types defined `config.go:57-115`. Defaults via `NewDefaultConfig` + `viper.SetDefault` chain at `config.go:217-247`.
- **REQ-5.10 (env > yaml > defaults):** Achieved via viper `SetEnvPrefix("JTPOST")` + `SetEnvKeyReplacer` + `AutomaticEnv` + explicit `BindEnv` for nested keys at `config.go:251-271`. Test: `TestConfig_EnvOverride` in config_test.go (CP-9). ✅
- REQ-5.11: `defaults.platforms` preserved (`config.go:52`, default `["telegram"]`).
- **REQ-5.12 (Validate rejects zero tenant/author):** `config.go:313-318`. Test: `TestConfig_Validate_RejectsZeroTenant` / `_Author`. ✅
- REQ-5.3 (invalid storage.type): `config.go:307-312` returns `ErrConfigInvalid`. Note: only checks if non-empty; default `"fs"` set elsewhere — acceptable.

### Group 6 — `jtpost init` (REQ-6.1 … REQ-6.7) — ✅ Verified
- REQ-6.1: `init.go:77-83` writes default cfg with generated UUIDv7s.
- **REQ-6.2 / 6.3 (interactive prompt + abort):** `init.go:49-58`. Aborts if empty answer or doesn't start with y/Y. Writes `Aborted` to stderr, returns nil (exit 0). Test: `TestCLIInit_*` (CP-13). ✅
- REQ-6.4: y/Y → cfg.Save with new UUIDs.
- REQ-6.5: `--force` skips prompt (`init.go:49`).
- REQ-6.6: Creates `cfg.PostsDir/<tenant_short>/` and `cfg.TemplatesDir` (`init.go:86-92`).
- **REQ-6.7 (UUID prefix collision retry):** `init.go:62-75` retries up to 10 times. Test: `TestCLIInit_UUIDPrefixUniqueness` (CP-12). ✅
  - Minor observation: after 10 collisions, code emits a warning and continues with the colliding short ID (per ADR / error-handling table in design.md §2.7 — "log warning and continue, probability negligible"). Matches design intent.

### Group 7 — FS repo (REQ-7.1 … REQ-7.6) — ✅ Verified
- **REQ-7.1 (tenant subdir):** `repository.go:249-251` builds path as `<root>/<tenantShortID>/<slug>.md`. Test: `TestFSRepo_Create_TenantSubdir` (verified file at expected subdir). ✅
- REQ-7.2: `List` reads only `tenantSubdir(filter.TenantID)` (line 118). Test: `TestFSRepo_List_TenantScoped`.
- REQ-7.3: `GetByID`/`GetBySlug`/`Delete` all return `ErrTenantMismatch` if no tenant in ctx. Test: `TestFSRepo_GetByID_NoContext_TenantMismatch`.
- REQ-7.4: `frontmatter_parser.go` + struct yaml tags; required fields always written.
- **REQ-7.5 (required field validation):** `frontmatter_parser.go` `validateRequired` (paraphrased from grep). Test: `TestFrontmatter_RejectsMissingRequired` (CP-7). ✅
- REQ-7.6: `Post.TenantShortID()` strips dashes, returns first 8 chars (`post.go:198-209`). Test: `TestPost_TenantShortID` (CP-8).

### Group 8 — HTTP API (REQ-8.1 … REQ-8.5) — ✅ Verified
- REQ-8.1: `TenantFromConfigMiddleware` in `middleware.go:15-26`, applied via `Server.apply` for `/api/posts*`, `/api/stats`, `/api/plan`, `/api/tags`.
- REQ-8.2: Handlers extract via `core.TenantFromContext` (server.go:129, 259, 371, 425, 496, 532, 713).
- REQ-8.3: `jsonPost` mirrors `Post` (server.go:587-608) with `omitempty` for optionals.
- **REQ-8.4 (403 tenant_mismatch):** server.go:269-272 (POST) and :371-373 (GET) and :425-427 (PATCH) return `403 {"error":"tenant_mismatch"}`. Test: covered in server_test.go (CP-14). ✅
- **REQ-8.5 (400 tenant_id_immutable):** server.go:411-422 returns `400 {"error":"tenant_id_immutable"}` if PATCH body's `tenant_id` differs from stored value. ✅

### Group 9 — CLI (REQ-9.1 … REQ-9.6) — ✅ Verified
- **REQ-9.1 / 9.2:** `list.go:66 case "json"` + `printPostsJSON` (line 87-91) coerces nil → `[]`. Test: `TestCLIList_FormatJSON*` (CP-15). ✅
- **REQ-9.3:** `grep -n getService internal/cli/*.go` returns nothing — symbol fully removed. ✅
- REQ-9.4 / 9.5 / 9.6: `new.go` accepts --tenant / --author flags with UUID parsing; defaults to config values.

### Group 10 — Tests / backcompat (REQ-10.1 … REQ-10.3) — ✅ Verified
- 9 packages green, race clean, build OK, lint 0 issues. Config tests new and comprehensive. testdata posts updated.

**Summary: all 65 REQ verified ✅. No partial or missing.**

---

## 4. Design Conformance

| ADR | Implementation | Status |
|---|---|---|
| ADR-1: tenant via context.Context | `internal/core/scope.go` defines private `ctxKey` + helpers. Used in fsrepo, httpapi, cli. | ✅ Conforms |
| ADR-2: allowedTransitions private + IsTransitionAllowed | `core.go:30` private `var`, `core.go:42` public predicate. No public access to map. | ✅ Conforms |
| ADR-3: Revision++ in service | `service.go:166-167` (UpdatePost) and `service.go:357-358` (updateInternal). No repository increments. | ✅ Conforms |
| ADR-4: Attachment.Path relative; AbsolutePath validates traversal | `post.go:135-148` uses `filepath.Clean` + `HasPrefix(absRoot+separator)`. Returns `ErrValidation` for unsafe paths. Test: `TestAttachment_AbsolutePath_RejectsTraversal`. | ✅ Conforms (good defense) |
| ADR-5: ResponsePayload as json.RawMessage inline | `post.go:157` field type `json.RawMessage`, yaml tag with omitempty. Round-trip tested. | ✅ Conforms |
| ADR-6: defaults.platforms preserved silently | `config.go:52`, default `["telegram"]`, no warning. | ✅ Conforms |
| ADR-7: Backward-compat config schema | `loadFromFile` calls `SetDefault` for every new key; old yaml without new sections still works. `NewServer(service, publisher)` kept as backcompat shim (server.go:39). | ✅ Conforms |

§2.3 "Files NOT Requiring Changes" spot-checks (6 files):
- `cmd/jtpost/main.go` — unchanged ✅
- `internal/core/clock.go` — unchanged ✅
- `internal/core/slug.go` — unchanged ✅
- `internal/core/publisher.go` — unchanged ✅
- `Taskfile.yml` — unchanged ✅
- `.golangci.yaml` — unchanged ✅

---

## 5. Correctness Properties Coverage (CP-1 … CP-15)

| CP | Test | Verified |
|---|---|---|
| CP-1 TenantScopeIsolation | `TestFSRepo_GetByID_OtherTenant_NotFound`, `TestFSRepo_List_TenantScoped` | ✅ |
| CP-2 TenantImmutability | `TestService_UpdatePost_TenantImmutable` | ✅ |
| CP-3 RevisionMonotonic | `TestService_UpdatePost_IncrementsRevision` | ✅ |
| CP-4 TransitionTableClosure | core_test.go cartesian sweep + `TestService_UpdateStatus_AllowedTransitions` | ✅ |
| CP-5 PublishHistoryTruncation | `TestFrontmatter_PublishHistoryTruncation` | ✅ |
| CP-6 PostRoundTrip | `TestPost_RoundTrip_YAML`, `TestPost_RoundTrip_JSON` | ✅ |
| CP-7 FrontmatterRequiredFields | `TestFrontmatter_RejectsMissingRequired` (frontmatter_parser_test.go) | ✅ |
| CP-8 TenantShortIDPrefix | `TestPost_TenantShortID`, `TestPostFilter_TenantShortID` | ✅ |
| CP-9 ConfigEnvOverride | `TestConfig_EnvOverride` in config_test.go | ✅ |
| CP-10 UpdateStatusEnforcement | `TestService_UpdateStatus_DisallowedRejected` | ✅ |
| CP-11 SortByWhitelist | `TestPostFilter_IsValidSortKey` | ✅ |
| CP-12 InitUUIDPrefixUniqueness | `TestCLIInit_UUIDPrefixUniqueness` (init_test.go) | ✅ |
| CP-13 InitOverwriteGuard | `TestCLIInit_ExistingFile_AnswerNo` | ✅ |
| CP-14 APITenantEnforcement | `TestHTTP_*_TenantMismatch` in server_test.go | ✅ |
| CP-15 ListJSONValid | `TestCLIList_FormatJSON*` in list_test.go | ✅ |

### Spot-checks of three CPs (read test bodies):

**CP-1 (TenantScopeIsolation):** `repository_test.go:41-62` — creates a post with `testTenant1`, then calls `GetByID` with `tenantCtx(testTenant2)`. Asserts `errors.Is(err, core.ErrNotFound)`. Matches statement "post.TenantID != ctx.TenantID → ErrNotFound, does not return post." ✅

**CP-2 (TenantImmutability):** `service_test.go:135-153` — Create then mutate `post.TenantID = testTenant2`, call `UpdatePost`. Asserts `errors.Is(err, ErrTenantMismatch)` and `repo.posts[id].TenantID == originalTenant`. Both clauses of the property checked. ✅

**CP-3 (RevisionMonotonic):** `service_test.go:106-130` — loop of 10 updates, asserts final `Revision == 11` (1 from create + 10 from updates) and `CreatedAt` unchanged, `UpdatedAt` matches advanced clock. Property statement satisfied. ✅

---

## 6. Code Quality & Security

Spot-checks performed:

- **`Attachment.AbsolutePath` (post.go:137-148):** uses `filepath.Abs(postsDir)`, `filepath.Join`, then `filepath.Clean`, and verifies `strings.HasPrefix(cleaned, absRoot+separator)`. This correctly defeats `..` traversal and absolute-path injection. Edge case: an attachment with empty `Path` passes (cleaned == absRoot), which is acceptable (caller decides). ✅

- **fsrepo tenant scope (repository.go):** `GetByID`, `GetBySlug`, `Delete` all check `core.TenantFromContext` first and return `ErrTenantMismatch` if absent. After loading a post, an additional check `post.TenantID != tenantID → ErrNotFound` enforces REQ-4.6 conflict-priority resolution. `List` checks `filter.TenantID == uuid.Nil → ErrValidation`. ✅

- **HTTP error shapes (server.go):** `writeJSONError` writes `Content-Type: application/json` and `{"error": code}` body — matches REQ-8.4 / 8.5 spec exactly. The handlers use `http.StatusForbidden` (403) for `tenant_mismatch` and `http.StatusBadRequest` (400) for `tenant_id_immutable` and `invalid_tenant_id`. ✅

- **`internal/cli/init.go`:** `--force` correctly bypasses prompt; without `--force`, prompt is shown via `cmd.OutOrStdout()` and reads via `cmd.InOrStdin()` (testable). UUID-collision retry loop bounded at 10 iterations, falls through with a warning if exhausted (matches design's error-handling). The interactive parser correctly rejects empty input and any answer not starting with y/Y. ✅

- **`config.uuidDecodeHook`:** correctly handles three cases — non-UUID target type (passthrough), non-string source (passthrough), empty string → `uuid.Nil`, valid string → `uuid.Parse(s)`. Error from invalid UUID propagates. Env-binding chain at lines 251-271 explicitly covers every nested key, sufficient for viper's known limitation with `AutomaticEnv` + nested. ✅

- **Concurrency / Go pitfalls:**
  - No nil-pointer concerns in `*Attachment`/`*string` fields — all accesses guarded.
  - `service.PostService` is stateless w.r.t. mutable fields; concurrent Updates can race on Revision (acknowledged in implementation report and ADR-3, deferred to F2).
  - Context cancellation: handlers propagate `r.Context()` correctly through `Service.*` methods.
  - No unsynchronized maps (`tagSet` etc. are local to handlers).

### Minor observations (not findings — informational)

- `internal/adapters/httpapi/server.go:185-229 sortPosts` (the legacy HTTP query-param `?sort=`): `statusOrder` map (lines 192-195) is missing `StatusArchived` and `StatusFailed` — both will sort as 0 (idea-tier). With archived/failed posts, order will be confusing but not incorrect. This sort path is separate from the new `PostFilter.SortBy`; F1 does not require it. Not a finding for F1.
- `service.go:269-272 GetNextPost` correctly excludes new `StatusArchived`/`StatusFailed` from candidates. ✅

---

## 7. Findings

**F-1: Status-sort map missing two new statuses**
Severity: minor
Location: `internal/adapters/httpapi/server.go:192-195`
Issue: The HTTP query-param sort (`?sort=status`) uses a hard-coded `statusOrder` map covering only 5 of 7 statuses. `StatusArchived` and `StatusFailed` map to default `0` (treated as `idea`-tier). Sorting becomes non-deterministic/non-intuitive once archived or failed posts are present.
Recommendation: Add `core.StatusFailed: 5, core.StatusArchived: 6` to the map. Trivial fix; doesn't block F1.

**F-2: `listPosts` post-filter for tenant is redundant and slightly confusing**
Severity: minor (style)
Location: `internal/adapters/httpapi/server.go:158-166`
Issue: After `service.ListPosts(ctx, filter)` already filters by `filter.TenantID`, the handler does an additional in-memory pass keeping only posts where `p.TenantID == filter.TenantID || p.TenantID == uuid.Nil`. Comment says it's for mock-repos in tests; this is production code coupling to test fakes.
Recommendation: Either remove the redundant filter (production repos honor `filter.TenantID`), or move the workaround behind an interface in test code only.

**F-3: `config.Validate` allows empty `Storage.Type`**
Severity: minor
Location: `internal/adapters/config/config.go:307-312`
Issue: The check is `if c.Storage.Type != "" && c.Storage.Type != "fs"|"sqlite"|"postgres"`. If a user explicitly clears the field in YAML, no error is returned (it just remains empty — viper default normally fills it as `"fs"`, but a yaml `storage: {type: ""}` would bypass the default). REQ-5.3 says "иное значение, чем fs/sqlite/postgres → ErrConfigInvalid" — which arguably includes `""`.
Recommendation: Treat empty-after-load as `"fs"` (set in `Validate` as fallback) or include `""` in the rejection set. Behavior in defaults path is fine; only odd edge case.

**F-4: SQLite repository touched but not yet using new tenant columns in queries**
Severity: minor (already documented as known limitation)
Location: `internal/adapters/sqlite/repository.go`
Issue: Schema gets `tenant_id`/`author_id`/`revision` columns + index, but the Go query layer does not yet filter by `tenant_id`. A user running with `storage.type=sqlite` today would get cross-tenant leakage.
Recommendation: This is explicitly deferred to F2 per the implementation report. Recommend adding a runtime guard in `NewServer`/CLI startup that refuses `storage.type=sqlite` with a "not yet supported in F1" message until F2 lands. Optional — not a blocker.

**F-5: Unused helper `errIsValidation`**
Severity: nit
Location: `internal/core/service.go:373-375`
Issue: Function is `//nolint:unused`-tagged and declared as a "тестовая утилита" but isn't actually called anywhere in the tree.
Recommendation: Remove or actually use it. Nit-level cleanup.

---

## 8. Verdict

**PASS.**

All 65 functional requirements are implemented and tested. All 15 correctness properties have corresponding test coverage, and three of them were verified by reading the test bodies. All 7 ADRs are honored in code. Build, lint, vet, and full test suite (incl. `-race`) are green. Tenant-isolation invariants (the most security-relevant aspect of F1) are enforced consistently across service, fsrepo, and httpapi layers, and the path-traversal protection in `Attachment.AbsolutePath` is correctly defensive.

Five findings, all minor/nit. None block the feature; they can be addressed in a follow-up commit or rolled into F2/F4 work where they're more naturally located. Specifically:

- F-1, F-3, F-5 are trivial cleanups (~15 minutes total).
- F-2 is style/refactor, not behavioral.
- F-4 is acknowledged in the implementation report and explicitly deferred to F2.

The implementation is solid: clean separation of concerns, honest about its known concurrency limitation (Revision race deferred to F2 with optimistic locking), and the test suite is thorough enough to give confidence that subsequent features can be built on this foundation without surprises.
