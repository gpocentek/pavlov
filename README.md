# Pavlov

Pavlov is a log watcher that reacts to patterns in log files. You define rules in a YAML config: each rule tails a file, matches lines with a regular expression, evaluates a condition, and runs an action when that condition is met.

Use it to alert on error spikes, fire a script when a specific failure appears, or detect when expected log lines stop showing up.

## Early adopter notice

**This project is not production-ready.** It is a personal side project — there is no roadmap or commitment beyond what is here today, and it may be reworked, shelved, or abandoned.

- **No releases yet.** There are no versioned binaries or tagged releases. Build from source (see below).
- **The config format will change.** Field names, structure, and supported options may break between commits. Treat your config as disposable until a stable release ships.
- **Limited test coverage and operational polish.** Error handling, logging, and edge cases are still being worked on.

If you try Pavlov, expect rough edges. Issues and pull requests are welcome, but will be handled as time permits.

> **Full disclosure:** Docs and code in this repo are being built with help from AI tools. They are enthusiastic, occasionally hallucinate, and have never read a log file in production.

## How it works

```
log file  →  tailer  →  regex match  →  condition  →  action
```

1. Pavlov tails one or more log files (multiple rules can share the same file).
2. Each incoming line is checked against each rule's `pattern` (Go regular expression).
3. If the line matches, the rule's `condition` decides whether to fire.
4. When the condition passes, the rule's `action` runs (log a message or execute a shell script).

Rules with an `absence` condition are also evaluated periodically (once per second) in addition to line matches.

## Requirements

- Go 1.26 or later (see [go.mod](go.mod))
- A Unix-like system with `fsnotify` support (macOS and Linux are the primary targets)

## Build

```bash
make build
```

This produces a `pavlov` binary in the project root. Alternatively:

```bash
go build -o pavlov ./cmd/pavlov
```

## Run

```bash
./pavlov -config /path/to/config.yaml
```

By default, Pavlov looks for `/etc/pavlov/config.yaml`. Override with `-config`.

Validate a config file without starting the daemon:

```bash
./pavlov -config config.yaml -check-config
```

Pavlov handles `SIGINT` and `SIGTERM` for graceful shutdown.

On shutdown, Pavlov:

1. Stops tailing log files and closes watchers.
2. Stops rule evaluators and cancels any in-flight actions (including shell scripts and their process groups).
3. Waits for all goroutines to finish, up to a configurable deadline.
4. Exits with status `1` if shutdown does not complete in time.

Configure the shutdown deadline with the `-shutdown-timeout` flag (default 10 seconds):

```bash
./pavlov -config config.yaml -shutdown-timeout 30s
```

Under normal conditions, shutdown completes in well under a second. The deadline exists as a safety net when a shell action hangs or a goroutine fails to exit.

### Logging

Set the log level with the `PAVLOV_LOG_LEVEL` environment variable:

| Value   | Level |
|---------|-------|
| `debug` | Debug |
| `info`  | Info (default) |
| `warn`  | Warn |
| `error` | Error |

Logs are written to stderr.

## Configuration

Pavlov reads a single YAML file. The top-level key is `rules`, a list of rule objects.

```yaml
rules:
  - name: my_rule
    file: /var/log/app/error.log
    pattern: 'error: (?P<service>\w+)'
    group_by: service
    cooldown: 30
    condition:
      type: threshold
      count: 5
      window: 60
    action:
      type: log
      template: "{{ .Rule }} fired for {{ .Group }}"
```

### Rule fields

| Field       | Required | Description |
|-------------|----------|-------------|
| `name`      | yes      | Unique identifier for the rule. Used in logs and action context. |
| `file`      | yes      | Path to the log file to tail. Resolved to an absolute path at load time. The file may not exist yet at startup, but its parent directory must exist. |
| `pattern`   | yes      | Go regular expression matched against each log line. Use named capture groups `(?P<name>...)` to extract values. |
| `condition` | yes      | When to fire after a match. See [Conditions](#conditions). |
| `action`    | yes      | What to do when the condition passes. See [Actions](#actions). |
| `group_by`  | no       | Name of a capture group from `pattern`. State (counters, cooldowns) is tracked per group value. The group name must appear in `pattern` as `(?P<group_by>...)`. |
| `cooldown`  | no       | Minimum seconds between firings for the same group. Default is `0` (no cooldown). Applies to all condition types. |

### Conditions

Every condition has a `type` field. Cooldown is configured on the rule, not inside the condition.

#### `match`

Fire on the first matching line (after cooldown).

```yaml
condition:
  type: match
```

Use for immediate alerts: "tell me the first time this error appears."

#### `threshold`

Fire when at least `count` matching lines arrive within a sliding `window` (seconds).

```yaml
condition:
  type: threshold
  count: 5   # minimum matches required
  window: 60     # sliding window in seconds
```

Use for rate-based alerts: "tell me when this error happens 5 times in a minute."

#### `absence`

Fire when no matching line has been seen for `duration` seconds.

```yaml
condition:
  type: absence
  duration: 120   # seconds without a match
```

Use for heartbeat / liveness checks: "tell me when `heartbeat ok` stops appearing."

Absence rules are evaluated periodically, once per second (not configurable yet). On startup, the clock starts from the current time — a missing heartbeat will not fire until `duration` seconds have passed without a match.

### Actions

Every action has a `type` field. All action types also accept optional execution controls:

| Field            | Default | Description |
|------------------|---------|-------------|
| `timeout`        | `0`     | Maximum seconds an action may run. `0` means no limit. When exceeded, the action's context is cancelled. For `shell` actions, the script and any child processes are killed. |
| `stop_previous`  | `false` | When `true`, a new firing cancels any still-running action from a previous firing before starting the new one. Cancellation is scoped per group when `group_by` is set; otherwise one in-flight action per rule. |

```yaml
action:
  type: shell
  script: ./scripts/alert.sh
  timeout: 10
  stop_previous: true
```

#### `log`

Write a formatted message to Pavlov's log output (stderr) at info level.

```yaml
action:
  type: log
  template: "rule={{ .Rule }} group={{ .Group }} line={{ .Line }}"
```

`template` is required. It uses Go's [text/template](https://pkg.go.dev/text/template) syntax with the fields below.

| Template field | Description |
|----------------|-------------|
| `.Rule`        | Rule name |
| `.File`        | Log file path |
| `.Line`        | Full matched log line (empty for absence-triggered actions) |
| `.Timestamp`   | Time of the event |
| `.GroupBy`     | `group_by` field value |
| `.Group`       | Captured group value (empty if `group_by` is not set) |
| `.Captures`    | Map of all named capture groups |

#### `shell`

Execute a script when the condition fires.

```yaml
action:
  type: shell
  script: ./scripts/alert.sh
```

`script` is required. It must be an existing, executable file path (validated at config load).

The script receives these environment variables:

| Variable | Description |
|----------|-------------|
| `PAVLOV_RULE` | Rule name |
| `PAVLOV_FILE` | Log file path |
| `PAVLOV_LINE` | Full matched log line |
| `PAVLOV_TS`   | Unix timestamp of the event |
| `PAVLOV_GROUP` | Captured group value |
| `PAVLOV_GROUP_BY` | `group_by` field value |
| `PAVLOV_CAPTURE_<name>` | One variable per named capture group (e.g. `PAVLOV_CAPTURE_backend`) |

Script stdout and stderr are captured in Pavlov's logs on failure. Timeouts and cancellation kill the script's entire process group (not just the top-level process), so child processes started by the script do not keep running after a timeout or `stop_previous` cancel.

## Example config

The repository includes [config.yaml](config.yaml) with three illustrative rules:

```yaml
rules:
  # Rate limit: fire when 5+ timeouts for the same backend occur in 10 seconds
  - name: upstream_timeout
    file: /var/log/nginx/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    group_by: backend
    condition:
      type: threshold
      count: 5
      window: 10
    cooldown: 30
    action:
      type: log
      template: "{{ .Rule }} {{ .File }} {{ .Line }} {{ .Timestamp }} {{ .Group }} {{ .Captures }}"

  # Immediate: fire once per backend when a connection failure is matched
  - name: connection_failed
    file: /var/log/nginx/error.log
    pattern: 'connect failed.*: (?P<backend>[0-9a-z.]+)'
    group_by: backend
    condition:
      type: match
    cooldown: 0
    action:
      type: shell
      script: ./scripts/run.sh
      timeout: 10
      stop_previous: true

  # Liveness: fire when "heartbeat ok" has not appeared for 10 seconds
  - name: heartbeat_missing
    file: /var/log/app.log
    pattern: 'heartbeat ok'
    condition:
      type: absence
      duration: 10
    cooldown: 60
    action:
      type: log
      template: "rule:{{ .Rule }} file:{{ .File }} timestamp:{{ .Timestamp }}"
```

## Behavior notes

- **Tail from end:** On startup, Pavlov begins reading each file from the end. Only new lines written after startup are evaluated.
- **File rotation:** Copytruncate rotation is detected and handled (the tailer rewinds). Create events on a new file are also handled.
- **Shared files:** Multiple rules can watch the same `file`. One tailer is created per unique file path.
- **Event buffering:** Each rule has an internal buffer (512 events). If it fills up, new lines are dropped with a warning.
- **Async actions:** Actions run in a separate goroutine so a slow script does not block log processing.

## Known limitations

- Single config file only (directory-based config loading is planned).
- Line parsing is regex-only (JSON and key=value parsers are planned).
- No HTTP, syslog, or other action types yet.
- Shell script paths are validated at load time but not re-checked at runtime.
- Absence check interval is hardcoded to one second.

Planned work is tracked in [TODO.md](TODO.md).

## Why "Pavlov"?

Named after Ivan Pavlov’s conditioning experiments: a stimulus (log line matches your `pattern`) eventually triggers a learned response (`action`), without you watching the file. You train the rules; Pavlov rings the bell.

## License

MIT — see [LICENSE](LICENSE).
