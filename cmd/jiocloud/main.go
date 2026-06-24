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
	"github.com/AmanDevelops/jiocloud/internal/parallel"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

// defaultParallel is the default number of concurrent transfers (cf. rclone --transfers).
const defaultParallel = 4

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
  jiocloud download <remotePath> [local] [-parallel N]
                                         Download a file or folder; folders download
                                         files concurrently (default N=4).
  jiocloud delete <remotePath>           Move a file or folder to the trash.
  jiocloud copy <dir> [remotePath] [-dry-run] [-parallel N]
                                         One-way copy of a local dir into a remote folder,
                                         creating folders and uploading files concurrently.
  jiocloud sync <dir> [remotePath] [-dry-run] [-parallel N]
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

func runCopy(args []string) { runCopyOrSync("copy", args, false) }

func runSync(args []string) { runCopyOrSync("sync", args, true) }

func runCopyOrSync(name string, args []string, deleteExtraneous bool) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "list what would change without uploading or creating folders")
	parallelN := fs.Int("parallel", defaultParallel, "number of files to upload concurrently")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "%s: usage: jiocloud %s <dir> [remotePath] [-dry-run] [-parallel N]\n", name, name)
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
	if err := copier.Run(api.New(creds), srcDir, remotePath, *dryRun, deleteExtraneous, *parallelN); err != nil {
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

	// ResolvePath returns the root folder (named "") for an empty path or "/".
	// Trashing the root would wipe the whole account, so refuse it.
	if obj.ObjectName == "" || obj.ParentObjectKey == "" {
		fatal(fmt.Errorf("refusing to delete the root folder; specify a path inside it"))
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
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	parallelN := fs.Int("parallel", defaultParallel, "number of files to download concurrently")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "download: usage: jiocloud download <remotePath> [local] [-parallel N]")
		os.Exit(2)
	}
	remotePath := fs.Arg(0)
	localPath := filepath.Base(remotePath)
	if fs.NArg() >= 2 {
		localPath = fs.Arg(1)
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

	switch obj.ObjectType {
	case api.TypeFile:
		fmt.Fprintf(os.Stderr, "Downloading file %s to %s...\n", remotePath, localPath)
		if err := client.Download(obj.ObjectKey, localPath); err != nil {
			fatal(err)
		}
		fmt.Printf("Successfully downloaded %s\n", localPath)
	case api.TypeFolder:
		fmt.Fprintf(os.Stderr, "Downloading folder %s to %s/...\n", remotePath, localPath)
		// Enumerate the tree (creating local dirs) first, then download files
		// concurrently — files are independent so this is a safe speed-up.
		tasks, err := collectDownloadTasks(client, obj.ObjectKey, localPath)
		if err != nil {
			fatal(err)
		}
		err = parallel.Run(tasks, *parallelN, func(t downloadTask) error {
			fmt.Fprintf(os.Stderr, "  %s\n", t.localPath)
			if err := client.Download(t.objectKey, t.localPath); err != nil {
				return fmt.Errorf("downloading %s: %w", t.localPath, err)
			}
			return nil
		})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Successfully downloaded %d files to %s/\n", len(tasks), localPath)
	default:
		fatal(fmt.Errorf("unknown object type %q", obj.ObjectType))
	}
}

type downloadTask struct {
	objectKey string
	localPath string
}

// collectDownloadTasks recursively walks a remote folder, creating the matching
// local directory tree and returning the flat list of files to download.
func collectDownloadTasks(client *api.Client, folderKey, localDir string) ([]downloadTask, error) {
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return nil, err
	}

	items, err := client.ListFolder(folderKey)
	if err != nil {
		return nil, err
	}

	var tasks []downloadTask
	for _, item := range items {
		itemLocalPath := filepath.Join(localDir, item.ObjectName)
		switch item.ObjectType {
		case api.TypeFolder:
			sub, err := collectDownloadTasks(client, item.ObjectKey, itemLocalPath)
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, sub...)
		case api.TypeFile:
			tasks = append(tasks, downloadTask{objectKey: item.ObjectKey, localPath: itemLocalPath})
		}
	}
	return tasks, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
