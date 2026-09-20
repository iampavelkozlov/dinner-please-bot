# AGENTS.md

## Repository overview

This repository contains a Go Telegram bot that serves recipes extracted from
`Любимые_рецепты_по_категориям_с_супами.pdf`. The bot has no database. Recipes
are stored as Telegram-ready Markdown files under `recipes/` and loaded when the
application starts.

The bot responds to a chat mention only when the sender's Telegram username is
present in the configured allowlist. The same allowlist applies to inline button
callbacks.

## Repository layout

- `cmd/bot/main.go` — composition root, signal handling, Telegram polling, and
  application startup.
- `config/config.yaml` — default runtime configuration. Secrets must be supplied
  through environment variables.
- `internal/config/` — YAML and environment configuration loading and validation.
- `internal/recipes/` — filesystem-backed recipe catalog loading.
- `internal/telegram/` — mention filtering, authorization, inline keyboards,
  pagination, message replacement, and recipe delivery.
- `recipes/<number>-<category>/<number>.md` — one recipe per Markdown file.
- `Dockerfile` — production container image.

## Architecture rules

- Keep `cmd/bot/main.go` limited to dependency wiring and process lifecycle.
- Keep Telegram-specific behavior in `internal/telegram`.
- Keep filesystem catalog behavior in `internal/recipes`.
- Keep configuration loading and validation in `internal/config`.
- Define narrow interfaces in the consuming package when external behavior must
  be mocked in tests.
- Do not add a database, web framework, dependency injection framework, or other
  infrastructure unless a concrete requirement needs it.
- Prefer the standard library. Add third-party dependencies only when they
  materially simplify the implementation.

## Recipe data invariants

- Treat the source PDF text as immutable source material. Do not rewrite,
  correct, shorten, translate, or otherwise alter recipe wording.
- Formatting changes are allowed only to make the original text render clearly
  in Telegram Markdown.
- Keep exactly one dish in each `.md` file.
- Keep recipes grouped by category directories and preserve the numeric filename
  prefix so loading order remains deterministic.
- Every recipe must contain a formatted title, serving information, ingredients,
  and all preparation text present in the PDF.
- Preserve source notes about missing content or inconsistencies verbatim.
- A single Telegram message is limited to 4096 characters. Recipes up to 8192
  characters are sent in exactly two parts. Keep both parts within the limit and
  preserve their combined text exactly.
- If recipe extraction is changed, verify all 42 bundled recipe files against
  all corresponding PDF pages and confirm their message lengths.

## Telegram behavior invariants

- Ignore ordinary messages unless they mention the current bot username.
- Ignore mentions from users outside `bot.allowed_usernames`.
- Apply the same authorization check to every callback query; do not trust an
  inline button merely because the bot created it.
- Matching of Telegram usernames and the bot mention is case-insensitive.
- On a valid mention, send the category keyboard.
- On category selection, delete the previous bot message and send the recipe
  menu.
- Paginate recipe menus according to `bot.page_size` and delete the previous menu
  after every page transition.
- On recipe selection, delete the menu and send the recipe as Telegram Markdown.
- For a two-part recipe, attach the back button only to the second part. Returning
  to the menu must delete both recipe messages.
- Answer every callback query before doing longer work so Telegram clients do not
  leave the loading indicator active.
- Keep callback data compact and within Telegram's 64-byte limit. Validate all
  callback indices and identifiers before use.
- Never log the bot token or include it in errors.

## Configuration

Follow the configuration approach used by the Conduit reference repository:
checked-in YAML defaults, environment overrides through `cleanenv`, explicit
validation, and table-driven configuration tests.

Supported environment variables:

- `BOT_TOKEN` — required Telegram bot token.
- `BOT_ALLOWED_USERNAMES` — comma-separated usernames, with or without `@`.
- `BOT_RECIPES_PATH` — recipe root directory.
- `BOT_PAGE_SIZE` — recipes per menu page.
- `LOG_LEVEL` — `debug`, `info`, `warn`, or `error`.

Never commit a real bot token, `.env` file, or other credential. Keep the token
empty in the checked-in YAML configuration.

## Go implementation rules

- Use the Go version declared in `go.mod`.
- Apply the Modern Go Guidelines skill before writing or modifying Go code.
- Write idiomatic Go and format every changed Go file with `gofmt`.
- Wrap errors with operation context using `%w`.
- Use `log/slog` for runtime logs.
- Keep functions small and behavior explicit. Avoid speculative abstractions.
- Respect cancellation and graceful shutdown in process-level code.

## Testing rules

- Every change to handwritten behavior must add or update automated tests.
- Prefer table-driven tests with descriptive case names, following the Conduit
  repository style.
- Use `github.com/stretchr/testify/require` for assertions.
- Use in-memory fakes behind narrow interfaces for Telegram behavior. Unit tests
  must not contact Telegram or require a bot token.
- Cover successful paths, authorization failures, malformed callbacks, boundary
  values, dependency failures, and cleanup behavior.
- Recipe splitting tests must prove that:
  - each part is at most 4096 characters;
  - joining both parts reproduces the original Markdown exactly;
  - the back action removes both Telegram messages;
  - content longer than two Telegram messages is rejected explicitly.
- Keep tests deterministic and safe to run with the race detector.

## Required verification

After Go changes, run:

```bash
gofmt -w <changed-go-files>
go test -race ./...
go vet ./...
go build ./cmd/bot
go mod verify
```

After changes to the Dockerfile, configuration, runtime paths, or dependencies,
also build the image:

```bash
docker build -t dinner-please-bot:local .
```

After recipe changes, additionally confirm the catalog still contains the
expected category counts:

- Завтраки: 8
- Супы: 3
- Салаты: 9
- Гарниры: 14
- Мясо: 8
- Total: 42

## Working-tree safety

- Inspect `git status --short` before editing.
- Preserve unrelated user changes.
- Do not reset, discard, or overwrite existing work.
- Do not commit generated binaries, coverage files, temporary PDF renders, IDE
  metadata, `.env`, or credentials.
- Do not create a commit unless the user explicitly requests one.
