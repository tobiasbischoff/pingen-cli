# Pingen CLI

A small Go CLI for working with the Pingen API from your terminal. It supports
OAuth client-credentials auth, listing organisations, and managing letters.

## Requirements

- Go 1.20+
- A Pingen account and OAuth client credentials

## Build

```sh
go build -o ./bin/pingen-cli ./cmd/pingen-cli
```

Run without building:

```sh
go run ./cmd/pingen-cli --help
```

## Quickstart

By default the CLI targets **staging**. To use production credentials, pass
`--env production`.

```sh
./bin/pingen-cli \
  --env production \
  --client-id YOUR_CLIENT_ID \
  --client-secret-file /path/to/secret \
  org list
```

Fetch a token and save it (plus credentials) in the local config:

```sh
./bin/pingen-cli \
  --env production \
  --client-id YOUR_CLIENT_ID \
  --client-secret-file /path/to/secret \
  auth token --save --save-credentials
```

## Authentication

The CLI uses the OAuth **client_credentials** grant. The default scope is:

```
letter batch webhook organisation_read
```

Override it with `--scope` on `auth token` if needed.

## Configuration

Config file location:

- `$XDG_CONFIG_HOME/pingen/config.json`, or
- `~/.config/pingen/config.json`

You can set values via the CLI:

```sh
./bin/pingen-cli config set env production
./bin/pingen-cli config set organisation_id YOUR_ORG_UUID
```

Environment variable overrides:

- `PINGEN_ENV`
- `PINGEN_API_BASE`
- `PINGEN_IDENTITY_BASE`
- `PINGEN_ORG_ID`
- `PINGEN_ACCESS_TOKEN`
- `PINGEN_CLIENT_ID`
- `PINGEN_CLIENT_SECRET`

## Common Commands

List organisations:

```sh
./bin/pingen-cli org list
```

List letters for a specific organisation:

```sh
./bin/pingen-cli --org YOUR_ORG_UUID letters list
```

Create a letter (upload PDF, optional auto-send):

```sh
./bin/pingen-cli --org YOUR_ORG_UUID letters create \
  --file ./letter.pdf \
  --address-position left \
  --auto-send
```

Send a letter (requires delivery options):

```sh
./bin/pingen-cli --org YOUR_ORG_UUID letters send LETTER_UUID \
  --delivery-product fast \
  --print-mode simplex \
  --print-spectrum color
```

## Output

Use `--json` for raw JSON output or `--plain` for human-friendly output. The
CLI defaults to plain text.

## Security Notes

- Avoid passing secrets directly on the command line (shell history). Prefer
  `--client-secret-file` or environment variables.
- Rotate credentials if they were exposed.
- Staging and production are separate environments with separate credentials.

## Development

Run tests (none currently, but keep this wired in):

```sh
go test ./...
```
