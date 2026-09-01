# Setup Reference

First-time setup is handled by the mosoo installer.

Run:

```sh
curl -fsSL https://install.mosoo.ai/install.sh | bash
```

For an auditable install flow:

```sh
curl -fsSL https://install.mosoo.ai/install.sh -o install-mosoo.sh
less install-mosoo.sh
bash install-mosoo.sh
```

By default, the installer is interactive and asks for `y` or `n` before each
high-impact step, such as installing the CLI, installing the Skill, writing
config, or running login.

For automation:

```sh
curl -fsSL https://install.mosoo.ai/install.sh | bash -s -- --yes
```

For an execution preview:

```sh
curl -fsSL https://install.mosoo.ai/install.sh | bash -s -- --dry-run
```

Then verify:

```sh
mosoo --version
mosoo doctor --json
```

If the mosoo CLI is already installed, first-time cloud setup does not require
the installer or any hostname input:

```sh
mosoo setup
mosoo auth login
mosoo doctor --json
```

`mosoo setup` stores the cloud service root (`https://cloud.mosoo.ai`). The CLI
derives the console API (`/api`) and Public API (`/api/v1`) hosts internally.
`mosoo auth login` saves one credential for both hosts.

For self-hosted or local targets, use the explicit setup subcommands:

```sh
mosoo setup self-host --base-url https://mosoo.example.com
mosoo setup local
```

The installer may install or update the mosoo CLI, install or update this mosoo
Skill, guide login, write initial config, and run readiness checks. Public
installs default to the mosoo cloud target at `https://cloud.mosoo.ai`; pass
`--target local` for a local development API.

Release installers can set `MOSOO_CLI_VERSION` to make the installer fail when the
installed binary does not report the expected `mosoo --version` build metadata.
