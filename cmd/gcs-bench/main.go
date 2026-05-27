package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const usage = `gcs-bench — GCS encrypted file system CLI

Usage:
  gcs-bench setup    --bucket B --prefix P             First-time setup
  gcs-bench admin    [add-user|remove-user|list-users]  Manage users
  gcs-bench daemon   [start|stop|status]                Manage daemon
  gcs-bench find     --name "..."                       Find objects by name
  gcs-bench read     --key "file.md"                    Read object content
  gcs-bench search   --query "..."                      Full-text search
  gcs-bench create   --name "file.md" --body "..."      Create new object
  gcs-bench update   --key "file.md" --body "..." [--expect-gen N]  Update with CAS

Config: ~/.config/gcs-bench/config.json (or GCS_BUCKET / GCS_PREFIX env vars)
All commands output minimal JSON with _elapsed_ms timing.`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	var err error
	cmd := os.Args[1]

	switch cmd {
	case "setup":
		err = adminSetup(os.Args[2:])
	case "admin":
		err = runAdmin(os.Args[2:])
	case "daemon":
		err = runDaemon(os.Args[2:])
	case "find":
		ensureDaemon()
		err = runFindViaDaemon(os.Args[2:])
	case "read":
		ensureDaemon()
		err = runReadViaDaemon(os.Args[2:])
	case "search":
		ensureDaemon()
		err = runSearchViaDaemon(os.Args[2:])
	case "create":
		ensureDaemon()
		err = runCreateViaDaemon(os.Args[2:])
	case "update":
		ensureDaemon()
		err = runUpdateViaDaemon(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", err.Error())
		os.Exit(1)
	}
}

// ensureDaemon auto-starts the daemon if it's not running
func ensureDaemon() {
	if isDaemonRunning() {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exe, "daemon", "start")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Start()
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	for i := 0; i < 50; i++ {
		if isDaemonRunning() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// --- Daemon-routed client commands with permission checks ---

func runFindViaDaemon(args []string) error {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	name := fs.String("name", "", "object name to search for")
	fs.Parse(args)
	out, err := daemonGet("/find", map[string]string{"name": *name})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runReadViaDaemon(args []string) error {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	key := fs.String("key", "", "object key")
	fs.Parse(args)

	if result := validateAccess(*key, "read"); !result.allowed {
		return outputDenied("read", *key, result.reason)
	}

	out, err := daemonGet("/read", map[string]string{"key": *key})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runSearchViaDaemon(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	query := fs.String("query", "", "search query")
	fs.Parse(args)
	out, err := daemonGet("/search", map[string]string{"query": *query})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runCreateViaDaemon(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "object name")
	body := fs.String("body", "", "content")
	fs.Parse(args)

	if result := validateAccess(*name, "create"); !result.allowed {
		return outputDenied("create", *name, result.reason)
	}

	out, err := daemonPost("/create", map[string]string{"name": *name, "body": *body})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runUpdateViaDaemon(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	key := fs.String("key", "", "object key")
	body := fs.String("body", "", "content to append")
	expectGen := fs.Int64("expect-gen", 0, "expected generation for CAS")
	replace := fs.Bool("replace", false, "replace instead of append")
	fs.Parse(args)

	if result := validateAccess(*key, "update"); !result.allowed {
		return outputDenied("update", *key, result.reason)
	}

	out, err := daemonPost("/update", map[string]any{
		"key":        *key,
		"body":       *body,
		"expect_gen": *expectGen,
		"replace":    *replace,
	})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func outputDenied(op, key, reason string) error {
	out := map[string]any{
		"status": "denied",
		"error":  "permission denied: " + reason,
		"key":    key,
	}
	data, _ := json.Marshal(out)
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
	return nil
}
