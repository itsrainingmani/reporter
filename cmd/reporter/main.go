package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Cached notifier availability (computed once).
var (
	notifierOnce   sync.Once
	notifierPath   string
	notifierExists bool
)

func main() {
	thresholdStr := flag.String("threshold", "10s", "minimum duration before a notification is sent (e.g. 5s, 1m30s)")
	always := flag.Bool("always", false, "send a notification even if the command completes before the threshold")
	title := flag.String("title", "Task finished", "title to display in notifications")
	silentBell := flag.Bool("no-bell", false, "do not emit a terminal bell alongside the notification")
	notifyOnly := flag.Bool("notify-only", false, "skip running a command and just send a notification (used by shell hooks)")
	commandStr := flag.String("cmd", "", "command string to display in notifications (notify-only mode)")
	durationStr := flag.String("duration", "", "duration of the already-finished command (notify-only mode)")
	exitFlag := flag.Int("exit", 0, "exit code of the already-finished command (notify-only mode)")
	pushURL := flag.String("push-url", getenvDefault("REPORTER_PUSH_URL", ""), "HTTP endpoint for phone push notifications (e.g. ntfy topic URL)")
	showVersion := flag.Bool("version", false, "print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] -- <command> [args...]\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("reporter %s\n", Version)
		os.Exit(0)
	}

	threshold, err := time.ParseDuration(*thresholdStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid threshold: %v\n", err)
		os.Exit(2)
	}

	if *notifyOnly {
		if *durationStr == "" {
			fmt.Fprintln(os.Stderr, "-duration is required in notify-only mode")
			os.Exit(2)
		}
		duration, err := time.ParseDuration(*durationStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid duration: %v\n", err)
			os.Exit(2)
		}
		cmdText := *commandStr
		if cmdText == "" {
			cmdText = strings.Join(flag.Args(), " ")
		}
		exitCode := notifyOnlyMode(cmdText, duration, *exitFlag, threshold, *always, *title, !*silentBell, *pushURL)
		os.Exit(exitCode)
	}

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	args := flag.Args()
	exitCode := runWithNotification(args, threshold, *always, *title, !*silentBell, *pushURL)
	os.Exit(exitCode)
}

func runWithNotification(args []string, threshold time.Duration, always bool, title string, bell bool, pushURL string) int {
	start := time.Now()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Set up signal forwarding to child process.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start command: %v\n", err)
		return 1
	}

	// Forward signals to child process in a goroutine.
	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	err := cmd.Wait()
	signal.Stop(sigChan)
	close(sigChan)

	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "failed to run command: %v\n", err)
			return 1
		}
	}

	if shouldNotify(duration, threshold, always) {
		if bell {
			fmt.Fprint(os.Stderr, "\a")
		}
		notify(title, strings.Join(args, " "), duration, exitCode, pushURL)
	}

	return exitCode
}

func notifyOnlyMode(command string, duration time.Duration, exitCode int, threshold time.Duration, always bool, title string, bell bool, pushURL string) int {
	if shouldNotify(duration, threshold, always) {
		if bell {
			fmt.Fprint(os.Stderr, "\a")
		}
		notify(title, command, duration, exitCode, pushURL)
	}
	return exitCode
}

func shouldNotify(duration, threshold time.Duration, always bool) bool {
	if always {
		return true
	}
	return duration >= threshold
}

func notify(title string, command string, duration time.Duration, exitCode int, pushURL string) {
	status := "succeeded"
	if exitCode != 0 {
		status = fmt.Sprintf("failed (exit %d)", exitCode)
	}

	body := fmt.Sprintf("%s in %s", status, formatDuration(duration))
	subtitle := command

	if err := notifyDesktop(title, body, subtitle); err != nil {
		// Graceful fallback to stderr if the platform notifier is unavailable.
		fmt.Fprintf(os.Stderr, "[notify] %s — %s\n", subtitle, body)
	}

	if err := pushToPhone(pushURL, title, body, subtitle); err != nil {
		fmt.Fprintf(os.Stderr, "[push] %v\n", err)
	}
}

func notifyDesktop(title, body, subtitle string) error {
	switch runtime.GOOS {
	case "darwin":
		return notifyMac(title, body, subtitle)
	case "linux":
		return notifyLinux(title, body, subtitle)
	default:
		return fmt.Errorf("no notifier available for %s", runtime.GOOS)
	}
}

func pushToPhone(url, title, body, subtitle string) error {
	if url == "" {
		return nil
	}

	payload := fmt.Sprintf("%s — %s\n%s", title, body, subtitle)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating request for %s: %w", url, err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("push to %s returned %s", url, resp.Status)
	}

	return nil
}

func initNotifier() {
	notifierOnce.Do(func() {
		switch runtime.GOOS {
		case "darwin":
			notifierPath, _ = exec.LookPath("osascript")
		case "linux":
			notifierPath, _ = exec.LookPath("notify-send")
		}
		notifierExists = notifierPath != ""
	})
}

func notifyMac(title, body, subtitle string) error {
	initNotifier()
	if !notifierExists {
		return fmt.Errorf("osascript not found in PATH")
	}
	script := fmt.Sprintf(`display notification "%s" with title "%s" subtitle "%s"`,
		escapeForAppleScript(body), escapeForAppleScript(title), escapeForAppleScript(subtitle))
	return exec.Command(notifierPath, "-e", script).Run()
}

func notifyLinux(title, body, subtitle string) error {
	initNotifier()
	if !notifierExists {
		return fmt.Errorf("notify-send not found in PATH")
	}
	message := fmt.Sprintf("%s — %s", subtitle, body)
	return exec.Command(notifierPath, title, message).Run()
}

func escapeForAppleScript(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", " ",
		"\r", " ",
		"\t", " ",
	)
	return replacer.Replace(s)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	// Keep it human-friendly while avoiding allocations from String().
	seconds := int(d.Round(time.Second).Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	switch {
	case hours > 0:
		return fmt.Sprintf("%dh%02dm%02ds", hours, minutes, secs)
	case minutes > 0:
		return fmt.Sprintf("%dm%02ds", minutes, secs)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

func getenvDefault(key, value string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return value
}
