package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runAdmin(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gcs-bench admin [add-user|remove-user|list-users|setup]")
	}

	switch args[0] {
	case "add-user":
		return adminAddUser(args[1:])
	case "remove-user":
		return adminRemoveUser(args[1:])
	case "list-users":
		return adminListUsers(args[1:])
	case "setup":
		return adminSetup(args[1:])
	default:
		return fmt.Errorf("unknown admin command: %s", args[0])
	}
}

func adminSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	bkt := fs.String("bucket", "", "GCS bucket name")
	userPrefix := fs.String("prefix", "", "your user/team prefix")
	project := fs.String("project", "", "GCP project ID")
	shared := fs.String("shared", "shared", "comma-separated shared prefixes (read-only)")
	fs.Parse(args)

	if *bkt == "" || *userPrefix == "" {
		return fmt.Errorf("--bucket and --prefix are required")
	}

	if *project == "" {
		out, err := exec.Command("gcloud", "config", "get-value", "project").Output()
		if err == nil {
			*project = strings.TrimSpace(string(out))
		}
	}

	fmt.Printf("Setting up gcs-bench...\n")
	fmt.Printf("  Bucket:     %s\n", *bkt)
	fmt.Printf("  Prefix:     %s\n", *userPrefix)
	fmt.Printf("  Project:    %s\n", *project)
	fmt.Printf("  Shared:     %s\n", *shared)

	sharedList := strings.Split(*shared, ",")
	for i := range sharedList {
		sharedList[i] = strings.TrimSpace(sharedList[i])
	}

	cfg := &appConfig{
		Bucket:         *bkt,
		UserPrefix:     *userPrefix,
		SharedPrefixes: sharedList,
		AgentID:        "",
		Credentials:    filepath.Join(configDir(), "sa-key.json"),
	}

	hostname, _ := os.Hostname()
	user := os.Getenv("USER")
	if user != "" {
		cfg.AgentID = fmt.Sprintf("%s@%s", user, hostname)
	}

	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("  Config:     %s\n", configFilePath())

	// Smoke test: try to connect
	ctx := context.Background()
	client, err := gcsClient(ctx)
	if err != nil {
		fmt.Printf("\n  Warning: could not connect to GCS: %v\n", err)
		fmt.Printf("  Place your SA key at %s or run 'gcloud auth application-default login'\n", credentialsPath())
	} else {
		defer client.Close()
		_, err := bucket(client).Attrs(ctx)
		if err != nil {
			fmt.Printf("\n  Warning: bucket %s not accessible: %v\n", *bkt, err)
		} else {
			fmt.Printf("\n  Bucket accessible.\n")
		}
	}

	fmt.Printf("\nSetup complete. Config saved to %s\n", configFilePath())
	return nil
}

func adminAddUser(args []string) error {
	fs := flag.NewFlagSet("add-user", flag.ExitOnError)
	user := fs.String("user", "", "username for the new user")
	userPrefix := fs.String("prefix", "", "GCS prefix this user can access")
	project := fs.String("project", "", "GCP project ID")
	bkt := fs.String("bucket", "", "GCS bucket name (defaults to config)")
	shared := fs.String("shared", "shared", "comma-separated shared read-only prefixes")
	fs.Parse(args)

	if *user == "" || *userPrefix == "" {
		return fmt.Errorf("--user and --prefix are required")
	}

	if *bkt == "" {
		*bkt = bucketName()
	}
	if *bkt == "" {
		return fmt.Errorf("--bucket is required (no config found)")
	}

	if *project == "" {
		out, err := exec.Command("gcloud", "config", "get-value", "project").Output()
		if err == nil {
			*project = strings.TrimSpace(string(out))
		}
	}
	if *project == "" {
		return fmt.Errorf("--project is required")
	}

	saName := fmt.Sprintf("gcs-bench-%s", *user)
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", saName, *project)

	fmt.Printf("Adding user: %s\n", *user)
	fmt.Printf("  SA:      %s\n", saEmail)
	fmt.Printf("  Prefix:  %s/\n", *userPrefix)
	fmt.Printf("  Bucket:  %s\n", *bkt)

	// 1. Create service account
	fmt.Printf("\n[1/4] Creating service account...\n")
	cmd := exec.Command("gcloud", "iam", "service-accounts", "create", saName,
		"--display-name", fmt.Sprintf("gcs-bench: %s (%s)", *user, *userPrefix),
		"--project", *project)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  Warning: SA may already exist: %v\n", err)
	}

	// 2. Grant objectUser on user prefix
	fmt.Printf("\n[2/4] Granting objectUser on %s/...\n", *userPrefix)
	condition := fmt.Sprintf(
		"expression=resource.name.startsWith('projects/_/buckets/%s/objects/%s/'),title=%s-rw",
		*bkt, *userPrefix, *user)
	cmd = exec.Command("gcloud", "storage", "buckets", "add-iam-policy-binding",
		fmt.Sprintf("gs://%s", *bkt),
		"--member", fmt.Sprintf("serviceAccount:%s", saEmail),
		"--role", "roles/storage.objectUser",
		"--condition", condition)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("grant objectUser: %w", err)
	}

	// 2b. Grant objectViewer on shared prefixes
	sharedList := strings.Split(*shared, ",")
	for _, sp := range sharedList {
		sp = strings.TrimSpace(sp)
		if sp == "" {
			continue
		}
		fmt.Printf("       Granting objectViewer on %s/ (read-only)...\n", sp)
		condition := fmt.Sprintf(
			"expression=resource.name.startsWith('projects/_/buckets/%s/objects/%s/'),title=%s-%s-ro",
			*bkt, sp, *user, sp)
		cmd = exec.Command("gcloud", "storage", "buckets", "add-iam-policy-binding",
			fmt.Sprintf("gs://%s", *bkt),
			"--member", fmt.Sprintf("serviceAccount:%s", saEmail),
			"--role", "roles/storage.objectViewer",
			"--condition", condition)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}

	// 3. Create SA key
	fmt.Printf("\n[3/4] Generating SA key...\n")
	userConfigDir := filepath.Join(configDir(), "users", *user)
	os.MkdirAll(userConfigDir, 0700)
	keyPath := filepath.Join(userConfigDir, "sa-key.json")
	cmd = exec.Command("gcloud", "iam", "service-accounts", "keys", "create", keyPath,
		"--iam-account", saEmail)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create key: %w", err)
	}

	// 4. Write user config
	fmt.Printf("\n[4/4] Writing user config...\n")
	userCfg := &appConfig{
		Bucket:         *bkt,
		UserPrefix:     *userPrefix,
		SharedPrefixes: sharedList,
		AgentID:        *user,
		Credentials:    keyPath,
	}
	userCfgPath := filepath.Join(userConfigDir, "config.json")
	data, _ := json.MarshalIndent(userCfg, "", "  ")
	os.WriteFile(userCfgPath, data, 0600)

	fmt.Printf("\nUser %s added.\n", *user)
	fmt.Printf("  SA key:  %s\n", keyPath)
	fmt.Printf("  Config:  %s\n", userCfgPath)
	fmt.Printf("\nTo onboard this user, copy to their machine:\n")
	fmt.Printf("  1. %s → ~/.config/gcs-bench/sa-key.json\n", keyPath)
	fmt.Printf("  2. %s → ~/.config/gcs-bench/config.json\n", userCfgPath)
	fmt.Printf("\nOr send them this command:\n")
	fmt.Printf("  gcs-bench setup --bucket %s --prefix %s\n", *bkt, *userPrefix)

	return nil
}

func adminRemoveUser(args []string) error {
	fs := flag.NewFlagSet("remove-user", flag.ExitOnError)
	user := fs.String("user", "", "username to remove")
	project := fs.String("project", "", "GCP project ID")
	fs.Parse(args)

	if *user == "" {
		return fmt.Errorf("--user is required")
	}

	if *project == "" {
		out, err := exec.Command("gcloud", "config", "get-value", "project").Output()
		if err == nil {
			*project = strings.TrimSpace(string(out))
		}
	}

	saName := fmt.Sprintf("gcs-bench-%s", *user)
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", saName, *project)

	fmt.Printf("Removing user: %s (SA: %s)\n", *user, saEmail)

	cmd := exec.Command("gcloud", "iam", "service-accounts", "delete", saEmail,
		"--quiet", "--project", *project)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("delete SA: %w", err)
	}

	userConfigDir := filepath.Join(configDir(), "users", *user)
	os.RemoveAll(userConfigDir)

	fmt.Printf("User %s removed. SA deleted, local config cleaned up.\n", *user)
	return nil
}

func adminListUsers(args []string) error {
	usersDir := filepath.Join(configDir(), "users")
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		fmt.Println("No users configured.")
		return nil
	}

	fmt.Printf("Configured users:\n\n")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfgPath := filepath.Join(usersDir, e.Name(), "config.json")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			fmt.Printf("  %s (no config)\n", e.Name())
			continue
		}
		var cfg appConfig
		json.Unmarshal(data, &cfg)
		fmt.Printf("  %-20s prefix=%-20s bucket=%s\n", e.Name(), cfg.UserPrefix, cfg.Bucket)
	}
	return nil
}
