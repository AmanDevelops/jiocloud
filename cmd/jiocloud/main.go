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
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
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
  jiocloud download <remotePath> [local] [-parallel N] Download a file or folder to localPath.
  jiocloud delete <remotePath>           Move a file or folder to the trash.
  jiocloud copy <dir> [remotePath] [-dry-run] [-parallel N]
                                         One-way copy of a local dir into a remote folder,
                                         creating folders and uploading new/changed files.
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
	posArgs := parseInterspersed(fs, args)

	if len(posArgs) < 1 {
		fmt.Fprintln(os.Stderr, "upload: missing file argument")
		os.Exit(2)
	}
	path := posArgs[0]

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}

	client := api.New(creds)
	fmt.Fprintf(os.Stderr, "Uploading %s...\n", path)
	res, err := client.Upload(path, *folder, nil)
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

func parseInterspersed(fs *flag.FlagSet, args []string) []string {
	var posArgs []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			break
		}
		args = fs.Args()
		if len(args) > 0 {
			posArgs = append(posArgs, args[0])
			args = args[1:]
		}
	}
	return posArgs
}

func runCopy(args []string) {
	fs := flag.NewFlagSet("copy", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "list what would change without uploading or creating folders")
	parallelN := fs.Int("parallel", 6, "number of concurrent uploads")
	posArgs := parseInterspersed(fs, args)

	if len(posArgs) < 1 {
		fmt.Fprintln(os.Stderr, "copy: usage: jiocloud copy <dir> [remotePath] [-dry-run] [-parallel N]")
		os.Exit(2)
	}
	srcDir := posArgs[0]
	remotePath := ""
	if len(posArgs) >= 2 {
		remotePath = posArgs[1]
	}

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}
	if err := copier.Run(api.New(creds), srcDir, remotePath, *dryRun, false, *parallelN); err != nil {
		fatal(err)
	}
}

func runSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "list what would change without uploading or creating folders")
	parallelN := fs.Int("parallel", 6, "number of concurrent uploads")
	posArgs := parseInterspersed(fs, args)

	if len(posArgs) < 1 {
		fmt.Fprintln(os.Stderr, "sync: usage: jiocloud sync <dir> [remotePath] [-dry-run] [-parallel N]")
		os.Exit(2)
	}
	srcDir := posArgs[0]
	remotePath := ""
	if len(posArgs) >= 2 {
		remotePath = posArgs[1]
	}

	creds, err := config.Load()
	if err != nil {
		fatal(err)
	}
	if err := copier.Run(api.New(creds), srcDir, remotePath, *dryRun, true, *parallelN); err != nil {
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
	parallelN := fs.Int("parallel", 6, "number of concurrent downloads")
	posArgs := parseInterspersed(fs, args)

	if len(posArgs) < 1 {
		fmt.Fprintln(os.Stderr, "download: missing remote path argument")
		os.Exit(2)
	}
	remotePath := posArgs[0]
	localPath := filepath.Base(remotePath)
	if len(posArgs) >= 2 {
		localPath = posArgs[1]
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

	var tasks []downloadTask
	if obj.ObjectType == api.TypeFile {
		tasks = append(tasks, downloadTask{
			objectKey:  obj.ObjectKey,
			objectName: obj.ObjectName,
			localPath:  localPath,
			size:       obj.SizeInBytes,
		})
	} else if obj.ObjectType == api.TypeFolder {
		fmt.Fprintf(os.Stderr, "Collecting files from folder %s...\n", remotePath)
		tasks, err = collectDownloadTasks(client, obj.ObjectKey, localPath)
		if err != nil {
			fatal(err)
		}
	} else {
		fatal(fmt.Errorf("unknown object type %q", obj.ObjectType))
	}

	if len(tasks) == 0 {
		fmt.Println("No files to download.")
		return
	}

	p := mpb.New(mpb.WithWidth(60))
	err = parallel.Run(tasks, *parallelN, func(t downloadTask) error {
		name := t.objectName
		if len(name) > 35 {
			name = "..." + name[len(name)-32:]
		}

		bar := p.New(t.size,
			mpb.BarStyle(),
			mpb.PrependDecorators(
				decor.Name(name, decor.WCSyncSpaceR),
				decor.CountersKibiByte("% .2f / % .2f", decor.WCSyncSpace),
			),
			mpb.AppendDecorators(
				decor.Percentage(decor.WCSyncSpace),
			),
		)

		var last int64
		progress := func(downloaded, total int64) {
			if downloaded > last {
				bar.IncrBy(int(downloaded - last))
				last = downloaded
			}
		}

		if err := client.Download(t.objectKey, t.localPath, progress); err != nil {
			bar.Abort(false)
			return fmt.Errorf("downloading %s: %w", t.objectName, err)
		}
		
		// If total wasn't known beforehand (e.g. 0 size from list)
		bar.SetTotal(last, true)
		return nil
	})
	p.Wait()
	if err != nil {
		fatal(err)
	}

	fmt.Printf("Successfully downloaded %d files to %s\n", len(tasks), localPath)
}

type downloadTask struct {
	objectKey  string
	objectName string
	localPath  string
	size       int64
}

func collectDownloadTasks(client *api.Client, folderKey, localDir string) ([]downloadTask, error) {
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return nil, err
	}

	items, err := client.ListFolder(folderKey)
	if err != nil {
		return nil, err
	}

	var tasks []downloadTask
	for _, item := range items {
		itemLocalPath := filepath.Join(localDir, item.ObjectName)
		if item.ObjectType == api.TypeFolder {
			subTasks, err := collectDownloadTasks(client, item.ObjectKey, itemLocalPath)
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, subTasks...)
		} else if item.ObjectType == api.TypeFile {
			tasks = append(tasks, downloadTask{
				objectKey:  item.ObjectKey,
				objectName: item.ObjectName,
				localPath:  itemLocalPath,
				size:       item.SizeInBytes,
			})
		}
	}
	return tasks, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
