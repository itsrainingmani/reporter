# Reporter: finish-time notifications for long terminal tasks

![Vibe Coded](https://img.shields.io/badge/vibe_coded-100%25-green?logo=claude)

`reporter` wraps any command, measures its runtime, and sends a desktop notification when it finishes. It streams stdout/stderr directly so you can keep working in the same terminal and get a ping only when it matters.

An optional shell hook makes this automatic: every command you run triggers a notification when it crosses the threshold. You can also send a push to your phone through any HTTP endpoint (e.g. an [ntfy](https://ntfy.sh/) topic).

## Installation

### Install script

```bash
curl -sSL https://raw.githubusercontent.com/itsrainingmani/reporter/main/install.sh | bash
```

Then add to your shell rc:

```bash
source ~/.local/share/reporter/reporter-auto.sh
```

### From source

```bash
git clone https://github.com/itsrainingmani/reporter.git
cd reporter
make install
```

## Why this approach

- Lightweight single binary built with the Go standard library.
- Uses native notifiers: `osascript` on macOS, `notify-send` on Linux. Falls back to stderr if unavailable.
- No output buffering; runs your command in-place and preserves exit codes.
- Sensible defaults with a threshold so short commands do not spam notifications.

## Usage

```
reporter [flags] -- <command> [args...]
```

Flags:

- `-threshold 10s` minimum duration before notifying (e.g. `5s`, `1m30s`).
- `-always` notify even if the run was shorter than the threshold.
- `-title "Task finished"` custom notification title.
- `-no-bell` disable the terminal bell that accompanies the notification.
- `-version` print version and exit.

Examples:

- `reporter -- sleep 15`
- `reporter -threshold 30s -- go test ./...`
- `reporter -always -title "Deploy" -- bash -lc "make deploy && make smoke"`

Exit codes match the wrapped command; notifications include success/failure and elapsed time.

### Automatic mode (no manual trigger)

Source the shell hook once (e.g. in `~/.zshrc` or `~/.bashrc`):

```
source /path/to/reporter/shell/reporter-auto.sh
```

Environment knobs (set before sourcing):

- `REPORTER_THRESHOLD` duration string (default `10s`).
- `REPORTER_ALWAYS=1` to notify regardless of duration.
- `REPORTER_PUSH_URL` HTTP endpoint for phone pushes (see below).
- `REPORTER_BIN` path to the built binary if it is not on `$PATH`.
- `REPORTER_EXCLUDE` comma-separated list of commands to skip (e.g. `ls,cd,pwd,echo`).

The hook records every command’s start/end time, then calls `reporter -notify-only` in the background. No user action is required per command.

### Phone push notifications

Provide any HTTP endpoint via `REPORTER_PUSH_URL` or `-push-url`. A simple option is an ntfy topic:

```
export REPORTER_PUSH_URL="https://ntfy.sh/your-topic"
reporter -- sleep 15
```

The payload is a short text body with title, status, duration, and the command string. If the push fails, it logs a terse `[push]` line to stderr and still delivers the desktop notification.

## Development

```bash
make build    # build binary
make test     # run tests
make install  # install to ~/.local/bin
```

## Notification behavior

- **macOS**: uses `osascript` to show a native notification.
- **Linux**: uses `notify-send` if available.
- **Fallback**: prints a concise status line to stderr and optionally rings the terminal bell.
