# Mosoo CLI Usage Aggregate

This file aggregates Mosoo CLI usage across example app development logs and
execution-audited Codex session records.

Included sources:

| Source | Count total | Method |
| --- | ---: | --- |
| `examples/abstract-joke-agent/mosoo-cli-usage.md` | 35 | Explicit `Count` column. |
| `examples/resume-optimizer-agent/mosoo-cli-usage.md` | 46 | Explicit `Count` column. |
| `stock-trading-agents` Codex session audit | 35 | Parsed actual `function_call -> exec_command` records from `/Users/xiaoke/.codex/sessions/2026/06/24/rollout-2026-06-24T22-58-11-019efa23-813a-70a1-811c-bd31375b0d3d.jsonl`, from the original stock-app goal through the turn before the first usage-count request. |

Not included in this aggregate:

| Source | Reason |
| --- | --- |
| `examples/ai-girlfriend-agent/mosoo-cli-usage.md` | Uses grouped `Calls`, not per-command counts. |
| Worker runtime HTTP calls to Mosoo public Thread API | Not CLI invocations. |
| Plain text, Markdown examples, command outputs, and this audit's own `rg/cat/node` commands | Not actual Mosoo CLI executions for the target app. |

Total counted CLI invocations: 116.

## Aggregated Counts

| CLI command | Count | Sources |
| --- | ---: | --- |
| `mosoo doctor --json` | 4 | abstract-joke, stock-trading |
| `mosoo search "create published agent" --json` | 1 | abstract-joke |
| `mosoo search "access token" --json` | 1 | abstract-joke |
| `mosoo search "<intent>" -o json` | 1 | resume-optimizer |
| `mosoo search "create agent app publish start run thread" --json --limit 12` | 1 | stock-trading |
| `mosoo commands --json` | 2 | abstract-joke, resume-optimizer |
| `mosoo commands show console user viewer --json` | 2 | abstract-joke, resume-optimizer |
| `mosoo commands show console apps app-list --json` | 2 | abstract-joke, resume-optimizer |
| `mosoo commands show console apps create-app --json` | 3 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo commands show console agents create-agent --json` | 3 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo commands show console agents publish-agent --json` | 3 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo commands show console credentials available-agent-models --json` | 2 | abstract-joke, resume-optimizer |
| `mosoo commands show console credentials vendor-credential-list --json` | 2 | abstract-joke, resume-optimizer |
| `mosoo commands show console credentials create-vendor-credential --json` | 2 | abstract-joke, resume-optimizer |
| `mosoo commands show public-thread-api threads create --json` | 3 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo commands show public-thread-api events list-events --json` | 2 | abstract-joke, resume-optimizer |
| `mosoo commands show console-rest access create --json` | 1 | abstract-joke |
| `mosoo commands show console-rest access list --json` | 1 | abstract-joke |
| `mosoo commands show console sessions start-agent-run --json` | 2 | resume-optimizer, stock-trading |
| `mosoo commands show console onboarding onboarding-bootstrap --json` | 1 | resume-optimizer |
| `mosoo commands show console skills app-skill-list --json` | 1 | resume-optimizer |
| `mosoo commands show console environments app-environment-list --json` | 1 | resume-optimizer |
| `mosoo commands show console environments environment --json` | 1 | resume-optimizer |
| `mosoo commands show console apps app-overview --json` | 1 | resume-optimizer |
| `mosoo commands show console skills create-skill-fork --json` | 1 | resume-optimizer |
| `mosoo commands show console skills skill-detail --json` | 1 | resume-optimizer |
| `mosoo commands show console agents update-agent-config --json` | 1 | stock-trading |
| `mosoo auth status --hostname http://127.0.0.1:8787/api` | 3 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo auth status --hostname http://127.0.0.1:8787/api/v1` | 1 | stock-trading |
| `mosoo -o json console user viewer` | 3 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo -o json console apps app-list --organization-id <id>` | 3 | abstract-joke, stock-trading |
| `mosoo -o json console apps create-app --input-name <name> --input-organization-id <id>` | 2 | abstract-joke, resume-optimizer |
| `mosoo -o json console onboarding onboarding-bootstrap --input-name <name>` | 1 | resume-optimizer |
| `mosoo -o json console credentials available-agent-models --app-id <id> --runtime-id openai-runtime` | 4 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo -o json console credentials vendor-credential-list --app-id <id>` | 4 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo console agents create-agent --help` | 2 | abstract-joke, stock-trading |
| `mosoo console apps create-app --help` | 1 | stock-trading |
| `mosoo console credentials create-vendor-credential --help` | 2 | abstract-joke, resume-optimizer |
| `mosoo auth --help` | 1 | stock-trading |
| `mosoo auth status --help` | 1 | stock-trading |
| `mosoo console agents update-agent-config --help` | 1 | stock-trading |
| `mosoo -o json console credentials create-vendor-credential --input-models <model>` | 1 | resume-optimizer |
| `mosoo -o json console credentials create-vendor-credential --input-app-id <id> --input-vendor-id openai --input-name <name> --input-api-key <env>` | 2 | abstract-joke, resume-optimizer |
| `mosoo -o json console agents create-agent --set <input fields>` | 1 | abstract-joke |
| `mosoo -o json console agents create-agent --input-* ... --set 'input.skillIds=[]'` | 1 | abstract-joke |
| `mosoo -o json console agents create-agent --input-skill-ids <candidate>` | 3 | abstract-joke |
| `mosoo -o json console agents create-agent --input-app-id <id> --input-kind pet --input-model gpt-5.4-mini --input-name <name> --input-prompt <prompt> --input-provider openai --input-runtime-id openai-runtime --input-skill-ids ''` | 1 | abstract-joke |
| `mosoo -o json console agents create-agent --file <input.json>` | 1 | resume-optimizer |
| `mosoo -o json console agents create-agent ...` | 3 | stock-trading |
| `mosoo -o json console agents publish-agent --input-app-id <id> --input-agent-id <id>` | 5 | abstract-joke, resume-optimizer, stock-trading |
| `mosoo -o json public-thread-api threads create --agent-id <id> --file <body.json>` | 2 | resume-optimizer, stock-trading |
| `mosoo -o json public-thread-api events list-events --thread-id <id> --limit <n>` | 15 | resume-optimizer, stock-trading |
| `mosoo -o json console agents update-agent-config ...` | 2 | stock-trading |
| `mosoo -o json console-rest access create --set label=<label>` | 1 | abstract-joke |
| `mosoo -o json console apps app-overview --app-id <id> --agent-limit <n> --credential-limit <n>` | 1 | resume-optimizer |
| `mosoo -o json console environments app-environment-list --app-id <id>` | 1 | resume-optimizer |
| `mosoo -o json console skills app-skill-list --app-id <id>` | 1 | resume-optimizer |

## Stock Trading Agent Resources

| Agent | ID |
| --- | --- |
| Market Watch / 盯盘员 | `01KVX31WG6V652682P9ET34CXM` |
| Information Desk / 信息面 | `01KVX32398F4FH3EM7PKZT9AGZ` |
| Decision Lead / 决策员 | `01KVX323NRFZN294CX993HFHHC` |
