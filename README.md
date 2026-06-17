# mosoo-cli-go

Generated Go CLI for the Mosoo Public Thread API.

## Build

```sh
make build
```

This clones or updates the Mosoo repository under `.cache/mosoo`, exports the
Public Thread API OpenAPI document, runs Lathe code generation, and builds
`bin/mosoo`.

## Install

```sh
make install
```

By default, installation uses `go env GOBIN`, or `$(go env GOPATH)/bin` when
`GOBIN` is empty. Override the destination with `BINDIR`:

```sh
make install BINDIR="$HOME/.bin"
```

## Authenticate

Create an access token from the local Mosoo web app:

```text
http://localhost:5173/settings/access-tokens
```

Then log in against the local API:

```sh
mosoo auth login --hostname http://127.0.0.1:8787/api/v1
```

Paste the generated access token when prompted.

## Common Commands

```sh
mosoo search threads --json
mosoo commands --json
mosoo commands show public-thread-api threads create --json
```

Use `commands show` before executing an unfamiliar generated command so flags,
body shape, auth, and output format are explicit.
