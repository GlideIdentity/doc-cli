package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `gdrive-bench — Google Drive benchmark CLI

Usage:
  gdrive-bench auth                                  Authenticate with Google Drive
  gdrive-bench daemon [start|stop|status]            Manage persistent daemon
  gdrive-bench find   --name "..."  [--folder ID]    Find files by name
  gdrive-bench read   --id ID [--mime google-doc]     Read file content as plain text
  gdrive-bench search --query "..." [--folder ID]    Full-text search
  gdrive-bench create --name "..." --body "..." [--folder ID]  Create a new doc
  gdrive-bench update --id ID --body "..."           Update a document

When the daemon is running, all commands route through it automatically
(warm connection pool, no TLS/token overhead per call).

Set GDRIVE_FOLDER to scope all operations to a folder.`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	var err error
	cmd := os.Args[1]

	switch cmd {
	case "auth":
		err = runAuth()
	case "daemon":
		err = runDaemon(os.Args[2:])
	case "find":
		if isDaemonRunning() {
			err = runFindViaDaemon(os.Args[2:])
		} else {
			err = runFind(os.Args[2:])
		}
	case "read":
		if isDaemonRunning() {
			err = runReadViaDaemon(os.Args[2:])
		} else {
			err = runRead(os.Args[2:])
		}
	case "search":
		if isDaemonRunning() {
			err = runSearchViaDaemon(os.Args[2:])
		} else {
			err = runSearch(os.Args[2:])
		}
	case "create":
		if isDaemonRunning() {
			err = runCreateViaDaemon(os.Args[2:])
		} else {
			err = runCreate(os.Args[2:])
		}
	case "update":
		if isDaemonRunning() {
			err = runUpdateViaDaemon(os.Args[2:])
		} else {
			err = runUpdate(os.Args[2:])
		}
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

// --- Daemon-routed versions of each command ---

func runFindViaDaemon(args []string) error {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	name := fs.String("name", "", "file name to search for")
	folder := fs.String("folder", folderID(), "folder ID")
	fs.Parse(args)

	out, err := daemonGet("/find", map[string]string{"name": *name, "folder": *folder})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runReadViaDaemon(args []string) error {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	id := fs.String("id", "", "file ID to read")
	fs.String("mime", "", "ignored in daemon mode")
	fs.Parse(args)

	out, err := daemonGet("/read", map[string]string{"id": *id})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runSearchViaDaemon(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	query := fs.String("query", "", "search query")
	folder := fs.String("folder", folderID(), "folder ID")
	fs.Parse(args)

	out, err := daemonGet("/search", map[string]string{"query": *query, "folder": *folder})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runCreateViaDaemon(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "document name")
	body := fs.String("body", "", "document content")
	folder := fs.String("folder", folderID(), "parent folder ID")
	fs.Parse(args)

	out, err := daemonPost("/create", map[string]string{"name": *name, "body": *body, "folder": *folder})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runUpdateViaDaemon(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	id := fs.String("id", "", "file ID")
	body := fs.String("body", "", "content to append")
	replace := fs.Bool("replace", false, "replace instead of append")
	expect := fs.String("expect-modified", "", "fail if file's modifiedTime differs (optimistic concurrency)")
	fs.Parse(args)

	out, err := daemonPost("/update", map[string]any{
		"id":              *id,
		"body":            *body,
		"replace":         *replace,
		"expect_modified": *expect,
	})
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}
