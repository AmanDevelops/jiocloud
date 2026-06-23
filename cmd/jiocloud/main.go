package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmanDevelops/jiocloud/internal/api"
	"github.com/AmanDevelops/jiocloud/internal/config"
	"github.com/AmanDevelops/jiocloud/internal/copier"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

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
	case "download":
		runDownload(os.Args[2:])
	case "delete":
		runDelete(os.Args[2:])
	case "ls":
		runLs(os.Args[2:])
	case "mkdir":
		runMkdir(os.Args[2:])
	case "copy":
		runCopy(os.Args[2:])
	case "sync":
		runSync(os.Args[2:])
	case "whoami":
		runWhoami(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("jiocloud %s\n", version)
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
  jiocloud ls [remotePath]               List files and directories (defaults to root).
  jiocloud mkdir <remotePath>            Make the path if it doesn't already exist.
  jiocloud upload <file> [-folder KEY]   Upload a single file (auto small/chunked).
  jiocloud download <remotePath> [local] Download a file or folder to localPath.
  jiocloud delete <remotePath>           Move a file or folder to the trash.
  jiocloud copy <dir> [remotePath] [-dry-run]
                                         One-way copy of a local dir into a remote folder,
                                         creating folders and uploading new/changed files.
  jiocloud sync <dir> [remotePath] [-dry-run]
                                         Like copy, but deletes remote files/folders not present locally.
  jiocloud version                       Print the version.

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

func runCopy(args []string) {
	fs := flag.NewFlagSet("copy", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "list what would change without uploading or creating folders")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "copy: usage: jiocloud copy <dir> [remotePath] [-dry-run]")
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
	if err := copier.Run(api.New(creds), srcDir, remotePath, *dryRun, false); err != nil {
		fatal(err)
	}
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
	if err := copier.Run(api.New(creds), srcDir, remotePath, *dryRun, true); err != nil {
		fatal(err)
	}
}

func runDelete(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "delete: missing remote path argument")
		os.Exit(2)
	}
	remotePath := args[0]

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}

	client := api.New(creds)
	obj, err := client.ResolvePath(remotePath)
	if err != nil {
		fatal(fmt.Errorf("resolving path %q: %w", remotePath, err))
	}

	if obj.ObjectKey == "" {
		fatal(fmt.Errorf("cannot delete the root folder"))
	}

	fmt.Fprintf(os.Stderr, "Deleting %s (%s)...\n", remotePath, obj.ObjectType)
	if err := client.Trash(obj); err != nil {
		fatal(err)
	}
	fmt.Printf("Successfully moved %s to trash\n", remotePath)
}

func runLs(args []string) {
	remotePath := ""
	if len(args) > 0 {
		remotePath = args[0]
	}

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}

	client := api.New(creds)
	obj, err := client.ResolvePath(remotePath)
	if err != nil {
		fatal(fmt.Errorf("resolving path %q: %w", remotePath, err))
	}

	if obj.ObjectType != api.TypeFolder {
		fatal(fmt.Errorf("%q is not a folder", remotePath))
	}

	items, err := client.ListFolder(obj.ObjectKey)
	if err != nil {
		fatal(fmt.Errorf("listing folder: %w", err))
	}

	for _, item := range items {
		if item.ObjectType == api.TypeFolder {
			fmt.Printf("%12s %s/\n", "DIR", item.ObjectName)
		} else {
			fmt.Printf("%12d %s\n", item.SizeInBytes, item.ObjectName)
		}
	}
}

func runMkdir(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "mkdir: missing remote path argument")
		os.Exit(2)
	}
	remotePath := args[0]

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}

	client := api.New(creds)
	fmt.Fprintf(os.Stderr, "Making directory %s...\n", remotePath)
	obj, err := client.MkdirAll(remotePath)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("Directory ready: %s (key: %s)\n", remotePath, obj.ObjectKey)
}

func runDownload(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "download: missing remote path argument")
		os.Exit(2)
	}
	remotePath := args[0]
	localPath := filepath.Base(remotePath)
	if len(args) >= 2 {
		localPath = args[1]
	}

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}

	client := api.New(creds)
	obj, err := client.ResolvePath(remotePath)
	if err != nil {
		fatal(fmt.Errorf("resolving path %q: %w", remotePath, err))
	}

	if obj.ObjectType == api.TypeFile {
		fmt.Fprintf(os.Stderr, "Downloading file %s to %s...\n", remotePath, localPath)
		if err := client.Download(obj.ObjectKey, localPath); err != nil {
			fatal(err)
		}
		fmt.Printf("Successfully downloaded %s\n", localPath)
	} else if obj.ObjectType == api.TypeFolder {
		fmt.Fprintf(os.Stderr, "Downloading folder %s to %s/...\n", remotePath, localPath)
		if err := downloadFolderRecursive(client, obj.ObjectKey, localPath); err != nil {
			fatal(err)
		}
		fmt.Printf("Successfully downloaded folder to %s/\n", localPath)
	} else {
		fatal(fmt.Errorf("unknown object type %q", obj.ObjectType))
	}
}

func downloadFolderRecursive(client *api.Client, folderKey, localDir string) error {
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}

	items, err := client.ListFolder(folderKey)
	if err != nil {
		return err
	}

	for _, item := range items {
		itemLocalPath := filepath.Join(localDir, item.ObjectName)
		if item.ObjectType == api.TypeFolder {
			if err := downloadFolderRecursive(client, item.ObjectKey, itemLocalPath); err != nil {
				return err
			}
		} else if item.ObjectType == api.TypeFile {
			fmt.Fprintf(os.Stderr, "  %s\n", itemLocalPath)
			if err := client.Download(item.ObjectKey, itemLocalPath); err != nil {
				return fmt.Errorf("downloading %s: %w", item.ObjectName, err)
			}
		}
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
