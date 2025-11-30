package main

import (
	"testing"
	"time"
)

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		name      string
		duration  time.Duration
		threshold time.Duration
		always    bool
		want      bool
	}{
		{
			name:      "duration below threshold",
			duration:  5 * time.Second,
			threshold: 10 * time.Second,
			always:    false,
			want:      false,
		},
		{
			name:      "duration equals threshold",
			duration:  10 * time.Second,
			threshold: 10 * time.Second,
			always:    false,
			want:      true,
		},
		{
			name:      "duration above threshold",
			duration:  15 * time.Second,
			threshold: 10 * time.Second,
			always:    false,
			want:      true,
		},
		{
			name:      "always flag overrides threshold",
			duration:  1 * time.Second,
			threshold: 10 * time.Second,
			always:    true,
			want:      true,
		},
		{
			name:      "zero duration with always flag",
			duration:  0,
			threshold: 10 * time.Second,
			always:    true,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldNotify(tt.duration, tt.threshold, tt.always)
			if got != tt.want {
				t.Errorf("shouldNotify(%v, %v, %v) = %v, want %v",
					tt.duration, tt.threshold, tt.always, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "milliseconds",
			duration: 500 * time.Millisecond,
			want:     "500ms",
		},
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			want:     "2m30s",
		},
		{
			name:     "hours minutes seconds",
			duration: 1*time.Hour + 5*time.Minute + 3*time.Second,
			want:     "1h05m03s",
		},
		{
			name:     "exactly one hour",
			duration: 1 * time.Hour,
			want:     "1h00m00s",
		},
		{
			name:     "zero duration",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "sub-millisecond rounds to zero",
			duration: 100 * time.Microsecond,
			want:     "0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestEscapeForAppleScript(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special characters",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "double quotes",
			input: `say "hello"`,
			want:  `say \"hello\"`,
		},
		{
			name:  "backslash",
			input: `path\to\file`,
			want:  `path\\to\\file`,
		},
		{
			name:  "newline",
			input: "line1\nline2",
			want:  "line1 line2",
		},
		{
			name:  "carriage return",
			input: "line1\rline2",
			want:  "line1 line2",
		},
		{
			name:  "tab",
			input: "col1\tcol2",
			want:  "col1 col2",
		},
		{
			name:  "mixed special characters",
			input: "say \"hello\"\nwith\\path\tand\rtabs",
			want:  "say \\\"hello\\\" with\\\\path and tabs",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeForAppleScript(tt.input)
			if got != tt.want {
				t.Errorf("escapeForAppleScript(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetenvDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		setEnv       bool
		want         string
	}{
		{
			name:         "env not set returns default",
			key:          "TEST_REPORTER_UNSET_VAR",
			defaultValue: "default_value",
			setEnv:       false,
			want:         "default_value",
		},
		{
			name:         "env set returns env value",
			key:          "TEST_REPORTER_SET_VAR",
			defaultValue: "default_value",
			envValue:     "env_value",
			setEnv:       true,
			want:         "env_value",
		},
		{
			name:         "empty env returns default",
			key:          "TEST_REPORTER_EMPTY_VAR",
			defaultValue: "default_value",
			envValue:     "",
			setEnv:       true,
			want:         "default_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			}
			got := getenvDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getenvDefault(%q, %q) = %q, want %q",
					tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}
