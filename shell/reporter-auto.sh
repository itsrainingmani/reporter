#!/usr/bin/env bash
# Automatic terminal notifications via reporter.
# Source this file from your shell (zsh or bash) and the last command will trigger a notification when it runs longer than the threshold.

# Guard against multiple sourcing.
[[ -n "$_REPORTER_LOADED" ]] && return 0
_REPORTER_LOADED=1

: "${REPORTER_BIN:=$(command -v reporter)}"
: "${REPORTER_THRESHOLD:=10s}"
: "${REPORTER_ALWAYS:=}"
: "${REPORTER_PUSH_URL:=}"
# Comma-separated list of command prefixes to exclude from notifications.
# Example: REPORTER_EXCLUDE="ls,cd,pwd,echo,cat"
: "${REPORTER_EXCLUDE:=}"

if [[ -z "$REPORTER_BIN" ]]; then
  return 0 2>/dev/null || exit 0
fi

_reporter_cmd=""
_reporter_started=""
_reporter_started_ms=""
_reporter_guard=0

# Check if a command should be excluded from notifications.
_reporter_should_exclude() {
  local cmd="$1"
  local first_word="${cmd%% *}"

  [[ -z "$REPORTER_EXCLUDE" ]] && return 1

  if [[ -n "$ZSH_VERSION" ]]; then
    # Use zsh-native array splitting
    local -a patterns
    patterns=("${(@s:,:)REPORTER_EXCLUDE}")
    for pattern in "${patterns[@]}"; do
      # Trim whitespace
      pattern="${pattern#"${pattern%%[![:space:]]*}"}"
      pattern="${pattern%"${pattern##*[![:space:]]}"}"
      [[ "$first_word" == "$pattern" ]] && return 0
    done
  else
    # Bash: use IFS splitting
    local IFS=','
    for pattern in $REPORTER_EXCLUDE; do
      # Trim whitespace
      pattern="${pattern#"${pattern%%[![:space:]]*}"}"
      pattern="${pattern%"${pattern##*[![:space:]]}"}"
      [[ "$first_word" == "$pattern" ]] && return 0
    done
  fi
  return 1
}

# Get current time in milliseconds (with fallback to seconds).
_reporter_now_ms() {
  if [[ "$OSTYPE" == darwin* ]]; then
    # macOS: use perl for millisecond precision
    perl -MTime::HiRes=time -e 'printf "%d", time * 1000' 2>/dev/null || echo "$(($(date +%s) * 1000))"
  elif date --version >/dev/null 2>&1; then
    # GNU date supports %N for nanoseconds
    echo "$(date +%s%3N)"
  else
    # Fallback to seconds
    echo "$(($(date +%s) * 1000))"
  fi
}

_reporter_start() {
  # Avoid recursive triggers from our own functions.
  [[ $_reporter_guard -eq 1 ]] && return

  # In bash, DEBUG trap fires for every simple command in a compound statement.
  # Only record the FIRST command's timestamp; don't overwrite on subsequent ones.
  # This ensures "sleep 5 && echo done" measures from "sleep 5", not "echo done".
  if [[ -n "$BASH_VERSION" && -n "$_reporter_started" ]]; then
    return
  fi

  _reporter_cmd="$1"

  # Skip excluded commands early.
  if _reporter_should_exclude "$_reporter_cmd"; then
    _reporter_started=""
    return
  fi

  _reporter_started_ms="$(_reporter_now_ms)"
  _reporter_started="1"
}

_reporter_finish() {
  local last_exit=$?
  [[ -z "$_reporter_started" ]] && return

  local now_ms
  now_ms="$(_reporter_now_ms)"
  local dur_ms=$((now_ms - _reporter_started_ms))
  ((dur_ms < 0)) && dur_ms=0

  # Convert to duration string with millisecond precision.
  local dur_str
  if ((dur_ms < 1000)); then
    dur_str="${dur_ms}ms"
  else
    local dur_s=$((dur_ms / 1000))
    local dur_ms_rem=$((dur_ms % 1000))
    if ((dur_ms_rem == 0)); then
      dur_str="${dur_s}s"
    else
      dur_str="${dur_s}s${dur_ms_rem}ms"
    fi
  fi

  local args=(-notify-only -duration "$dur_str" -cmd "$_reporter_cmd" -exit "$last_exit" -threshold "$REPORTER_THRESHOLD")
  [[ -n "$REPORTER_ALWAYS" ]] && args+=(-always)
  [[ -n "$REPORTER_PUSH_URL" ]] && args+=(-push-url "$REPORTER_PUSH_URL")

  _reporter_guard=1
  # Subshell prevents job control messages from appearing in the terminal.
  ( "$REPORTER_BIN" "${args[@]}" >/dev/null 2>&1 & )
  _reporter_guard=0

  _reporter_started=""
}

if [[ -n "$ZSH_VERSION" ]]; then
  autoload -Uz add-zsh-hook
  add-zsh-hook preexec _reporter_start
  add-zsh-hook precmd _reporter_finish
elif [[ -n "$BASH_VERSION" ]]; then
  # Bash: use DEBUG trap for preexec and PROMPT_COMMAND for postcmd.
  # Only set DEBUG trap if not already trapping (avoid overwriting user's trap).
  _reporter_existing_debug_trap="$(trap -p DEBUG)"
  if [[ -z "$_reporter_existing_debug_trap" ]]; then
    trap '_reporter_start "$BASH_COMMAND"' DEBUG
  else
    # Chain with existing DEBUG trap by saving and executing it directly.
    # We store the full trap command and use eval to execute it, avoiding
    # fragile regex extraction that breaks on quoted strings.
    _reporter_old_debug_trap="$_reporter_existing_debug_trap"
    trap '_reporter_start "$BASH_COMMAND"; eval "$_reporter_old_debug_trap"' DEBUG
  fi
  unset _reporter_existing_debug_trap

  # Bash 5.1+ supports PROMPT_COMMAND as array, but only use array form if
  # PROMPT_COMMAND is already an array or unset. If it's a string, preserve
  # string form for compatibility with other tools.
  if [[ ${BASH_VERSINFO[0]} -gt 5 || (${BASH_VERSINFO[0]} -eq 5 && ${BASH_VERSINFO[1]} -ge 1) ]]; then
    if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
      # Already an array, prepend to it
      PROMPT_COMMAND=("_reporter_finish" "${PROMPT_COMMAND[@]}")
    elif [[ -z "${PROMPT_COMMAND:-}" ]]; then
      # Unset or empty, use array form for modern Bash
      PROMPT_COMMAND=("_reporter_finish")
    else
      # It's a string, keep it as a string for compatibility
      PROMPT_COMMAND="_reporter_finish; $PROMPT_COMMAND"
    fi
  else
    PROMPT_COMMAND="_reporter_finish${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
  fi
fi
