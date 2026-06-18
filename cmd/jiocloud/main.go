package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/AmanDevelops/jiocloud/internal/api"
	"github.com/AmanDevelops/jiocloud/internal/config"
	"github.com/AmanDevelops/jiocloud/internal/sync"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "login":
		runLogin(os.Args[2:])
	case "upload":
		runUpload(os.Args[2:])
	case "sync":
		runSync(os.Args[2:])
	case "whoami":
		runWhoami(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `jiocloud - minimal JioAiCloud CLI

Usage:
  jiocloud login [cookie]                Authenticate. If cookie is omitted you'll be prompted.
  jiocloud whoami                        Show the logged-in user and storage quota.
  jiocloud upload <file> [-folder KEY]   Upload a single file (auto small/chunked).
  jiocloud sync <dir> [remotePath] [-dry-run]
                                         One-way sync a local dir to a remote folder,
                                         creating folders and uploading new/changed files.

The login cookie format is:
  {{USER_ID}}:Basic {{AUTH_CODE}}:{{APP_SECRET}}:{{DEVICE_KEY}}
`)
}

func runLogin(args []string) {
	var cookie string
	if len(args) > 0 {
		cookie = strings.Join(args, " ")
	} else {
		fmt.Fprint(os.Stderr, "Paste login cookie: ")
		r := bufio.NewReader(os.Stdin)
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			fatal(err)
		}
		cookie = strings.TrimSpace(line)
	}

	fmt.Fprintln(os.Stderr, "Scraping default API credentials...")
	creds, err := api.Login(cookie)
	if err != nil {
		fatal(err)
	}
	if err := config.Save(creds); err != nil {
		fatal(err)
	}

	p, _ := config.Path()
	fmt.Printf("Logged in as %s\nCredentials saved to %s\n", creds.UserID, p)
}

func runUpload(args []string) {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)
	folder := fs.String("folder", "", "destination folder key (default: root)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "upload: missing file argument")
		os.Exit(2)
	}
	path := fs.Arg(0)

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}

	client := api.New(creds)
	fmt.Fprintf(os.Stderr, "Uploading %s...\n", path)
	res, err := client.Upload(path, *folder)
	if err != nil {
		fatal(err)
	}

	fmt.Printf("Uploaded %s (%d bytes)\n", res.ObjectName, res.SizeInByte)
	fmt.Printf("objectKey: %s\n", res.ObjectKey)
	if res.URL != "" {
		fmt.Printf("url: %s\n", res.URL)
	}
}

func runWhoami(args []string) {
	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}
	u, err := api.New(creds).UserInfo()
	if err != nil {
		fatal(err)
	}
	fmt.Printf("User:    %s (%s)\n", u.FirstName, u.UserID)
	fmt.Printf("Root:    %s\n", u.RootFolderKey)
	fmt.Printf("Storage: %d / %d bytes used\n", u.Quota.UsedSpace, u.Quota.AllocatedSpace)
}

func runSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "list what would change without uploading or creating folders")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "sync: usage: jiocloud sync <dir> [remotePath] [-dry-run]")
		os.Exit(2)
	}
	srcDir := fs.Arg(0)
	remotePath := ""
	if fs.NArg() >= 2 {
		remotePath = fs.Arg(1)
	}

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}
	if err := sync.Run(api.New(creds), srcDir, remotePath, *dryRun); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
