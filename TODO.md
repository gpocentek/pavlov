# TODO

Rough list of planned work. No timeline — items may be dropped or reordered. Maintained with help from AI tools (see README).

Items prefixed with **!** are important — reliability, safety, or operability gaps that should be addressed before wider use.

## Testing and docs

- [ ] Add documentation beyond README (config cookbook, architecture notes)
- [ ] Example configs per use case (nginx errors, k8s pod logs, heartbeat absence)
- [ ] `-version` flag (print embedded git commit / build time at startup)

## Configuration

- [ ] Load a directory of config files (merge all `*.yaml` into one ruleset)
- [ ] ! Config reload without restart (reload on `SIGHUP`)
- [ ] ! Rule dry-run mode (feed sample log lines through a rule; show match, captures, condition result, and what action would run — but do not execute it)
- [ ] YAML anchors / includes for shared action templates and repeated rule fragments
- [ ] Global defaults section in config (default `cooldown`, evaluator buffer size, shell timeout applied to all rules unless overridden)
- [ ] Environment variable substitution in config paths (e.g. `script: ${ALERT_SCRIPT}` expanded at load time)

## Logging

- [ ] Consistent structured log fields across components (rule name, file, group on every relevant message)
- [ ] Optional log-to-file output (in addition to stderr)
- [ ] Structured JSON log format option (for Loki, ELK, etc.)

## Parsers

Today every rule uses a Go regex on the raw log line. Planned alternatives:

- [ ] JSON (match on parsed fields instead of raw text)
- [ ] Key=value (`k=v`, match on extracted pairs)
- [ ] Per-rule regex flags (case-insensitive, multiline — without baking `(?i)` into every pattern)
- [ ] Invert match (treat non-matching lines as events; avoids fragile negative lookaheads in regex)
- [ ] Post-match filter (skip the line unless a capture equals a value, e.g. `backend == "api"`)

## Conditions

- [ ] ! Configurable ticker interval for `absence` (how often to check "still missing"; currently hardcoded to 1s)
- [ ] `rate` condition — cap repeat alerts **while a problem stays active** (distinct from `cooldown`, which only enforces a minimum gap *after* a fire):
  - `cooldown`: "I alerted you; stay quiet for N seconds, then you may alert again if the condition passes once more." During a sustained `threshold` breach, this can still page every cooldown period.
  - `rate`: "While the condition keeps being satisfied, alert at most once every N seconds." One reminder cadence for an ongoing incident, not one ping per re-qualifying event.
  - Example: errors never stop → `cooldown: 30` may fire at t=0, 30, 60…; `rate: 5m` fires at t=0, 5m, 10m… regardless of how often threshold re-triggers.
- [ ] Compound conditions (combine sub-conditions with AND/OR, e.g. `threshold` AND capture filter)
- [ ] Schedule windows (only evaluate or fire actions during configured hours/days)
- [ ] Cooldown scope option: apply cooldown per `group_by` value (current behaviour) or once for the whole rule across all groups

## Actions

- [ ] Syslog action (emit formatted alert to syslog instead of stderr)
- [ ] HTTP call action (POST/GET with headers, optional TLS skip, templated body)
- [ ] Load action templates from file (keep large templates out of the main config)
- [ ] Log action `file` field (write rendered template to a dedicated file; field exists in code but is unused today)
- [ ] Multiple actions per rule (run a list of actions when the condition fires)
- [ ] Idempotency key env var for shell actions (stable hash of rule + group + time bucket so downstream can dedupe)

## Tailer

- [ ] ! Tailer recovery (if a tailer goroutine exits with error, restart it with backoff instead of silently stopping that file)
- [ ] Rename/move rotation (handle `logrotate` rename-then-create, in addition to copytruncate and file-create)
- [ ] Multi-line and partial-line handling (very long lines, log lines without a trailing newline)

## Observability

- [ ] ! Dropped-event counters (increment when the 512-event evaluator buffer overflows; log periodic summary per rule)
- [ ] ! Metrics collection + Prometheus exporter (optional build tag, e.g. `-tags prometheus`; expose counters for lines processed, matches, fires, drops, tailer restarts, action errors)
- [ ] Health/status surface (HTTP `/health` or periodic log summary: tailers alive, per-rule last-fired time, enabled rule count)

## Build and deployment

- [ ] Conditional build tags to include/exclude parsers and actions (smaller binaries when only regex + shell are needed)
- [ ] systemd unit example (`Type=simple`, `ExecStart`, reload on `SIGHUP` once config reload exists)
- [ ] Docker image (minimal runtime, config mounted as a volume)
