# OpenAPI client + `--remote` mode (F5b) — Requirements

**Status:** Draft · **Branch:** `feature/openapi-client`

## Overview

F5b генерирует HTTP-client из `api/openapi.yaml` (созданного в F5) и подключает его к CLI как опциональный режим работы через `--remote URL --auth TOKEN`. Локальный режим (storage Bundle) остаётся default. Реализуется для proof-of-concept команды `jtpost list`. Server-side ServerInterface и остальные команды — F5c.

## Glossary

| Term | Definition | Code Artifact |
|------|------------|---------------|
| `apiclient` | Package с generated HTTP client | `internal/adapters/apiclient/client.gen.go` |
| `--remote URL` | Глобальный CLI-флаг для активации remote-mode | `internal/cli/root.go` |
| `--auth TOKEN` | Глобальный CLI-флаг с Bearer-token (PAT) | `internal/cli/root.go` |
| `JTPOST_AUTH_TOKEN` | Env-fallback для --auth | env var |
| `oapi-codegen-config-client.yaml` | Конфиг для client codegen | repo root |
| `newAPIClient` | Factory для apiclient.ClientWithResponses из cfg+flags | `internal/cli/remote.go` |

## User Stories

- Как **CLI-пользователь**, я хочу `jtpost list --remote https://jtpost.example.com --auth jtpat_...` чтобы получать список постов из удалённого сервиса.
- Как **разработчик**, я хочу env-fallback `JTPOST_AUTH_TOKEN` чтобы не передавать секрет каждый раз.
- Как **разработчик-сопровождающий**, я хочу что бы существующие local-mode команды (без `--remote`) НЕ ломались.

## Requirements

### Group 1 — Client codegen

**REQ-1.1** WHEN репозиторий содержит `oapi-codegen-config-client.yaml`, the system SHALL декларировать generate `client: true`, output `internal/adapters/apiclient/client.gen.go`, package `apiclient`, import-mapping types из `oapigen` package.

**REQ-1.2** WHEN команда `task generate:openapi:client` запускается, the system SHALL вызвать `oapi-codegen` с client-config и сгенерировать file. На повторных запусках — git-clean diff.

**REQ-1.3** WHEN команда `task generate:openapi` (deprecated/renamed в `:types`) запускается, the system SHALL по-прежнему генерировать types-only. Отдельная задача `:client` для клиентского кода.

**REQ-1.4** WHEN команда `task generate` запускается, the system SHALL запустить sqlc + types + client (3 sub-tasks).

**REQ-1.5** WHEN сгенерированный `client.gen.go` появляется, the system SHALL экспортировать `Client` (raw HTTP) и `ClientWithResponses` (typed responses) типы с методами для каждой operation из spec (включая `GetPostsWithResponse`, `LoginWithResponse`, и т.д.).

### Group 2 — CLI flags

**REQ-2.1** WHEN команда `jtpost <subcmd>` запускается, the system SHALL принимать глобальный `--remote URL` (string, optional) и `--auth TOKEN` (string, optional).

**REQ-2.2** WHEN env-переменная `JTPOST_AUTH_TOKEN` задана и `--auth` отсутствует, the system SHALL использовать env-value как auth-token.

**REQ-2.3** WHEN `--remote` задан и `--auth` пуст (и env-fallback пуст), the system SHALL вернуть ошибку `--auth required when using --remote`.

### Group 3 — newAPIClient helper

**REQ-3.1** WHEN модуль `internal/cli` экспортирует функцию `newAPIClient(cmd *cobra.Command) (*apiclient.ClientWithResponses, error)`, the system SHALL возвращать `nil, nil` если `--remote` не задан (signal к local-mode).

**REQ-3.2** WHEN `newAPIClient` вызывается с `--remote URL`, the system SHALL построить `apiclient.NewClientWithResponses(remoteURL, opts...)` где `opts` включают `apiclient.WithRequestEditorFn` для добавления `Authorization: Bearer <token>` header.

**REQ-3.3** WHEN `newAPIClient` сталкивается с invalid URL, the system SHALL вернуть ошибку с сообщением.

### Group 4 — `jtpost list --remote`

**REQ-4.1** WHEN команда `jtpost list --remote URL --auth TOKEN` запускается, the system SHALL построить apiclient, вызвать `client.GetPostsWithResponse(ctx, params)`, передать filter параметры (status/tag/search) если заданы, вывести posts через `printTable` или `printPostsJSON`.

**REQ-4.2** WHEN команда `jtpost list` запускается БЕЗ `--remote`, the system SHALL продолжить использовать local Bundle/repo (текущее поведение).

**REQ-4.3** WHEN remote API возвращает 401, the system SHALL вывести `unauthorized: invalid or expired token` и завершиться с exit 1.

**REQ-4.4** WHEN remote API возвращает 5xx или connection error, the system SHALL вывести error message и завершиться с exit 1.

### Group 5 — Тесты

**REQ-5.1** WHEN `task test` запускается, the system SHALL пройти `TestNewAPIClient_*` (URL parsing, auth-header injection).

**REQ-5.2** WHEN `task test` запускается, the system SHALL пройти `TestList_RemoteMode` через `httptest.NewServer` mock-server, возвращающий валидный JSON-ответ.

**REQ-5.3** WHEN `task test` запускается, the system SHALL пройти `TestList_LocalMode_Unchanged` (regression — без `--remote` существующее поведение).

## Verification Commands

| Action | Command |
|--------|---------|
| Test | `task test` |
| Build | `task build` |
| Generate | `task generate` |
| Generate (client only) | `task generate:openapi:client` |
