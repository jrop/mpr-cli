package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"golang.org/x/sync/semaphore"
)

func main() {
	cmd := func() *cobra.Command {
		// create the root cobra command: this is the one we will attach all of the
		// subcommands to
		cmd := &cobra.Command{
			Use: "mpr",
			RunE: func(cmd *cobra.Command, args []string) error {
				err := cmd.Help()
				os.Exit(1)
				return err
			},
		}

		cmd.AddCommand(&cobra.Command{
			Use:   "build <pkg>",
			Short: "Builds a package",
			Long:  `Builds a package. This is equivalent to running "makedeb" in the package's directory.`,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				pkgName := args[0]
				return runBuild(pkgName)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "check-stale",
			Short: "Checks for stale packages",
			Long:  `Checks for stale packages. A package is considered stale if it's version is behind repology's record.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runCheckStale()
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "clone <package-url>",
			Short: "Clones a package",
			Long:  `Clones a package. This is equivalent to running "git clone" in the packages directory.`,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				packageURL := args[0]
				return runClone(packageURL)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "each ...",
			Short: "Runs a command in each package's directory",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return fmt.Errorf("expected at least 1 argument, got 0")
				}
				return runEach(args)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "edit <package-name>",
			Short: "Edits a package's PKGBUILD",
			Long:  `Edits a package's PKGBUILD. This is equivalent to running "$EDITOR PKGBUILD" in the package's directory.`,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				pkgName := args[0]
				return runEdit(pkgName)
			},
		})

		cmd.AddCommand(func() *cobra.Command {
			// this subcommand will have its own flags, so we set it up inside of a
			// closure to avoid polluting the global flag set
			cmd := &cobra.Command{
				Use:   "install <package-url>",
				Short: "Installs a package",
				Long:  `Installs a package. This is equivalent to cloning and running "makepkg ..." in the package's directory.`,
				Args:  cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					packageURL := args[0]
					confirm, _ := cmd.Flags().GetBool("confirm")
					return runInstall(installArgs{
						packageURL: packageURL,
						confirm:    confirm,
					})
				},
			}
			cmd.Flags().BoolP("confirm", "c", true, "do not ask for confirmation")
			return cmd
		}())

		cmd.AddCommand(&cobra.Command{
			Use:   "list",
			Short: "Lists all packages",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runList()
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "outdated",
			Short: "Lists all outdated packages",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runOutdated()
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "reinstall <pkg>",
			Short: "Reinstalls a package",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				pkgName := args[0]
				return runReinstall(pkgName)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "update",
			Short: "Updates all packages (runs `git pull`)",
			Long:  `Updates all packages. This is equivalent to running "git fetch" in each package's directory.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runUpdate()
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "update-version <pkg> [new-version]",
			Short: "Updates the version of a package in a PKGBUILD file",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) < 1 || len(args) > 2 {
					return fmt.Errorf("expected at 1 or 2 arguments, got %d", len(args))
				}
				pkgName := args[0]
				newVersion := ""
				if len(args) == 2 {
					newVersion = args[1]
				}
				return runUpdateVersion(pkgName, newVersion)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "uninstall <pkg>",
			Short: "Uninstalls a package",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				pkgName := args[0]
				return runUninstall(pkgName)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "info <pkg>",
			Args:  cobra.ExactArgs(1),
			Short: "Shows information about a package",
			RunE: func(cmd *cobra.Command, args []string) error {
				pkgName := args[0]
				return runPkgInfo(pkgName)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "upgrade",
			Short: "Installs newly available versions",
			Long:  `Upgrades all packages. This is equivalent to running "makedeb ..." in each package's directory.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				confirm, _ := cmd.Flags().GetBool("confirm")
				return runUpgrade(upgradeArgs{
					confirm: confirm,
				})
			},
		})

		// return the root command
		return cmd
	}()

	// run the command
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

type installArgs struct {
	packageURL string
	confirm    bool
}

type upgradeArgs struct {
	confirm bool
}

func runBuild(pkgName string) error {
	cmd := mkcmd(true, "makedeb")
	cmd.Dir = mprDir(pkgName)
	return cmd.Run()
}

func runCheckStale() error {
	for _, fullPkgName := range listPackages() {
		fmt.Println("Checking " + fullPkgName + "...")

		printPkgMessage := func(msg ...any) {
			args := make([]any, len(msg)+1)
			args[0] = "=>"
			copy(args[1:], msg)
			fmt.Println(args...)
			fmt.Println()
		}

		pkgbuild := NewPKGBUILD(mprDir(fullPkgName))
		newestVersion, err := pkgbuild.getLatestRepologyPkgVersion()
		if err != nil {
			printPkgMessage(err)
			continue
		}
		if newestVersion == "SKIP" {
			printPkgMessage("skipping")
			continue
		}
		pkgver, err := pkgbuild.getSingleVariable("pkgver")
		if err != nil {
			printPkgMessage("could not read pkgver variables")
			continue
		}

		// remove quotes/single quotes from start/end:
		pkgver = strings.Trim(pkgver, "\"")
		pkgver = strings.Trim(pkgver, "'")

		green := color.New(color.FgGreen).SprintFunc()
		red := color.New(color.FgRed).SprintFunc()
		if newestVersion != pkgver {
			fmt.Println(" => current=" + red(pkgver) + ", latest=" + green(newestVersion))
			fmt.Println()
		}
	}
	return nil
}

func runClone(packageURL string) error {
	url := getPackageURL(packageURL)
	pkg := filepath.Base(url)
	if strings.Contains(pkg, ":") {
		pkg = strings.Split(pkg, ":")[1]
	}
	pkg = strings.TrimSuffix(pkg, ".git")

	for _, existingPkg := range listPackages() {
		if existingPkg == pkg {
			fmt.Println("package already exists")
			os.Exit(1)
		}
	}
	cmd := mkcmd(true, "git", "clone", url, pkg)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func runEach(args []string) error {
	for _, pkg := range listPackages() {
		fmt.Println("=> " + pkg)
		cmd := mkcmd(true, args[0], args[1:]...)
		cmd.Dir = mprDir(pkg)

		err := cmd.Run()
		if err != nil {
			return err
		}
		fmt.Println()
	}
	return nil
}

func runEdit(pkgName string) error {
	// spawn $EDITOR in the mpr directory:
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	dir := mprDir(pkgName)
	cmd := exec.Command(editor, filepath.Join(dir, "PKGBUILD"))
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runInstall(args installArgs) error {
	url := getPackageURL(args.packageURL)
	pkg := filepath.Base(url)
	err := runClone(args.packageURL)
	if err != nil {
		return err
	}

	if err := installMakedeb(); err != nil {
		return err
	}

	// open $EDITOR PKGBUILD:
	if args.confirm {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}
		cmd := exec.Command(editor, mprDir(pkg, "PKGBUILD"))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}

		// ask for confirmation:
		fmt.Print("Do you want to build the package now? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			os.RemoveAll(mprDir(pkg))
			os.Exit(1)
		}
	}

	var makedebArgs []string
	if !args.confirm {
		makedebArgs = append(makedebArgs, "-si", "--no-confirm")
	} else {
		makedebArgs = append(makedebArgs, "-si")
	}

	cmd := mkcmd(true, "makedeb", makedebArgs...)
	cmd.Dir = mprDir(pkg)
	err = cmd.Run()
	if err != nil {
		os.Exit(1)
	}

	err = updateMakedebInstallReceipt(pkg)
	if err != nil {
		return err
	}

	return nil
}

func runList() error {
	for _, pkg := range listPackages() {
		fmt.Println(pkg)
	}
	return nil
}

func runOutdated() error {
	var outdatedPkgs []string
	for _, pkg := range listPackages() {
		behind, err := isBehind(pkg)
		if err != nil {
			return err
		}
		if behind {
			outdatedPkgs = append(outdatedPkgs, pkg)
		}
	}
	for _, pkg := range outdatedPkgs {
		fmt.Println(pkg)
	}
	return nil
}

func runReinstall(pkgName string) error {
	cmd := mkcmd(true, "makedeb", "-si")
	cmd.Dir = mprDir(pkgName)
	return cmd.Run()
}

func runUpdate() error {
	packages := listPackages()

	// create an atomic counter:
	counter := 0
	failedPackages := make([]string, 0)

	lastLineLength := 0
	setLine := func(line string) {
		line = fmt.Sprintf("(%d/%d) %s", counter, len(packages), line)

		if len(line) < lastLineLength {
			fmt.Print("\r" + strings.Repeat(" ", lastLineLength))
		}
		lastLineLength = len(line)

		fmt.Print("\r" + line)
	}

	setLine("Updating")

	//
	// run 10 at a time in parallel:
	//
	// create a channel with a capacity of 10:

	ctx := context.TODO()
	MAX_WORKERS := 10
	sem := semaphore.NewWeighted(int64(MAX_WORKERS))
	// sem := make(chan bool, 10)

	for _, pkg := range packages {
		// acquire a slot:
		err := sem.Acquire(ctx, 1)
		if err != nil {
			panic(err)
		}

		go func(pkg string) {
			defer sem.Release(1)

			cmd := exec.Command("git", "pull")
			cmd.Dir = mprDir(pkg)
			// kill the command if it takes too long:
			timer := time.AfterFunc(10*time.Second, func() {
				if err := cmd.Process.Kill(); err != nil {
					panic(err)
				}
				setLine(fmt.Sprintf("Killed: %s (took too long)", pkg))
			})
			_, err := cmd.Output()
			counter++
			timer.Stop()
			if err != nil {
				setLine(fmt.Sprintf("Killed: %s (took too long)", pkg))
				failedPackages = append(failedPackages, pkg)
			}
			setLine(fmt.Sprintf("Updated %s", pkg))
		}(pkg)
	}

	// drain the channel:
	if err := sem.Acquire(ctx, int64(MAX_WORKERS)); err != nil {
		return err
	}

	fmt.Println()

	if len(failedPackages) > 0 {
		return fmt.Errorf("mpr update failed for some packages: %s", strings.Join(failedPackages, ", "))
	}
	return nil
}

func runUpdateVersion(pkgName string, newVersion string) error {
	dir := ""
	if pkgName == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = cwd
	} else {
		dir = mprDir(pkgName)
	}

	if newVersion == "" {
		pkgbuild := NewPKGBUILD(dir)
		latestRepologyVersion, err := pkgbuild.getLatestRepologyPkgVersion()
		if err != nil {
			return err
		}
		newVersion = latestRepologyVersion
	}

	pkgbuild := NewPKGBUILD(dir)
	err := pkgbuild.updateVar("pkgver", newVersion)
	if err != nil {
		return err
	}

	return nil
}

func runUninstall(pkgName string) error {
	installedPkgs := listPackages()
	if !stringSliceContainsString(installedPkgs, pkgName) {
		return fmt.Errorf("package %s is not installed", pkgName)
	}

	// uninstall the package:
	cmd := mkcmd(true, "sudo", "apt-get", "remove", pkgName)
	err := cmd.Run()
	if err != nil {
		return err
	}

	// remove the mpr directory:
	err = os.RemoveAll(mprDir(pkgName))
	if err != nil {
		return err
	}

	return nil
}

func runPkgInfo(pkgName string) error {
	pkgbuild := NewPKGBUILD(mprDir(pkgName))
	allVars, err := pkgbuild.getVariables()
	if err != nil {
		return err
	}
	for k, vals := range *allVars {
		for _, v := range vals {
			fmt.Printf("%s=%s\n", k, v)
		}
	}
	return nil
}

func runUpgrade(args upgradeArgs) error {
	for _, pkg := range listPackages() {
		behind, err := isBehind(pkg)
		if err != nil {
			return err
		}
		if !behind {
			continue
		}
		if err := installMakedeb(); err != nil {
			return err
		}

		var makedebArgs []string
		if !args.confirm {
			makedebArgs = append(makedebArgs, "-si", "--no-confirm")
		} else {
			makedebArgs = append(makedebArgs, "-si")
		}

		cmd := mkcmd(true, "makedeb", makedebArgs...)
		cmd.Dir = mprDir(pkg)
		err = cmd.Run()
		if err != nil {
			return err
		}

		err = updateMakedebInstallReceipt(pkg)
		if err != nil {
			return err
		}
	}
	return nil
}

func mprDir(segments ...string) string {
	mprDirEnv := os.Getenv("MPR_DIR")
	if mprDirEnv != "" {
		return filepath.Join(append([]string{mprDirEnv}, segments...)...)
	}

	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		panic(err)
	}
	userCacheDir = filepath.Join(userCacheDir, "mpr-packages")

	err = os.MkdirAll(userCacheDir, 0755)
	if err != nil {
		panic(err)
	}

	return filepath.Join(append([]string{userCacheDir}, segments...)...)
}

func mkcmd(loud bool, name string, arg ...string) *exec.Cmd {
	if loud {
		fmt.Printf("[#] %s ", name)
		for _, a := range arg {
			fmt.Printf("%s ", a)
		}
		fmt.Println()
	}
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = mprDir() // by default, commands are run in the mpr directory
	return cmd
}

func installMakedeb() error {
	_, err := exec.LookPath("makedeb")
	if err == nil {
		return nil
	}
	cmd := mkcmd(true, "bash", "-c", "wget -qO - 'https://shlink.makedeb.org/install' | MAKEDEB_RELEASE=makedeb bash -")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func listPackages() []string {
	// find all sub-directories in the mpr directory that:
	// 1. Contain a PKGBUILD file
	// 2. Contain a ".git" directory

	candidateFiles, err := ioutil.ReadDir(mprDir())
	if err != nil {
		log.Fatal(err)
	}

	var packages []string
	for _, entry := range candidateFiles {
		// the entry must be a directory:
		if !entry.IsDir() {
			continue
		}

		// the directory must itself contain a PKGBUILD file:
		if _, err := os.Stat(filepath.Join(mprDir(entry.Name()), "PKGBUILD")); err != nil {
			continue
		}

		// the directory must itself contain a .git directory:
		if _, err := os.Stat(filepath.Join(mprDir(entry.Name()), ".git")); err != nil {
			continue
		}

		// now read the PKGBUILD file and check if it contains a "pkgname" variable:
		pkgbuild := NewPKGBUILD(mprDir(entry.Name()))
		pkgname, err := pkgbuild.getSingleVariable("pkgname")
		if err != nil {
			continue
		}

		packages = append(packages, pkgname)
	}

	sort.Strings(packages)
	return packages
}

func getPkgHEADCommitHash(pkg string) (string, error) {
	var sbout, sberr strings.Builder
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Stdout = &sbout
	cmd.Stderr = &sberr
	cmd.Dir = mprDir(pkg)
	err := cmd.Run()

	if err != nil {
		if strings.Contains(sberr.String(), "not a git repository") {
			return "NOT_A_GIT_REPOSITORY", nil
		}

		return "", fmt.Errorf("could not get HEAD commit hash for package %s: %w", pkg, err)
	}
	return strings.TrimSpace(string(sbout.String())), nil
}

func updateMakedebInstallReceipt(pkg string) error {
	currentHash, err := getPkgHEADCommitHash(pkg)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(mprDir(pkg, ".git", "makedeb-install-receipt"), []byte(currentHash), 0644)
	if err != nil {
		return err
	}
	return nil
}

func readMakedebInstallReceipt(pkg string) (string, error) {
	currentReceipt, err := ioutil.ReadFile(mprDir(pkg, ".git", "makedeb-install-receipt"))

	// if the error is "no such file or directory", then the package has never
	// been installed:
	if err != nil && os.IsNotExist(err) {
		currentReceipt = []byte("")
		err = nil
	}
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(currentReceipt)), nil
}

func isBehind(pkg string) (bool, error) {
	currentHash, err := getPkgHEADCommitHash(pkg)
	if err != nil {
		return true, err
	}

	receiptHash, err := readMakedebInstallReceipt(pkg)
	if err != nil {
		return true, err
	}
	return (receiptHash != currentHash), nil
}

func getPackageURL(spec string) string {
	// if the spec is in USER/REPO format, assume it's a GitHub repo:
	matched, _ := regexp.MatchString(`^([^/:]+)/([^/:]+)$`, spec)
	if matched {
		return "https://github.com/" + spec
	}

	// if the spec is ID (case insensitive), assume it's an MPR package:
	matched, _ = regexp.MatchString(`(?i)^[a-z0-9_-]+$`, spec)
	if matched {
		return "https://mpr.makedeb.org/" + spec
	}

	return spec
}

func stringSliceContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
