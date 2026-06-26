# Setup Reference

First-time setup is handled by the Mosoo installer.

Run:

```sh
curl -fsSL https://install.mosoo.ai/codex | bash
```

For an auditable install flow:

```sh
curl -fsSL https://install.mosoo.ai/codex -o install-mosoo-codex.sh
less install-mosoo-codex.sh
bash install-mosoo-codex.sh
```

By default, setup is interactive and asks for `y` or `n` before each
environment-changing step, such as installing the CLI, installing the Mosoo
Skill, writing config, running login, or running optional readiness checks.

For automation:

```sh
curl -fsSL https://install.mosoo.ai/codex | bash -s -- --yes
```

For an execution preview:

```sh
curl -fsSL https://install.mosoo.ai/codex | bash -s -- --dry-run
```

Then verify:

```sh
mosoo --version
mosoo doctor --json
```

The installer may install or update the Mosoo CLI, install or update this Mosoo
Skill, guide login, write initial config, and run readiness checks. Public setup
defaults to the Mosoo cloud target at `https://api.mosoo.ai`; pass
`--target local` for a local development API. Cloudflare and Wrangler are not
default prerequisites for basic setup.

Automation can set `MOSOO_CLI_VERSION` to require a specific CLI build. Setup
fails if the installed binary does not report the expected `mosoo --version`
metadata.
