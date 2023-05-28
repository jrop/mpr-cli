package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
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
			cmd.Flags().BoolP("confirm", "c", false, "do not ask for confirmation")
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
			Short: "Updates all packages",
			Long:  `Updates all packages. This is equivalent to running "git fetch" in each package's directory.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runUpdate()
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "update-sums <pkg>",
			Args:  cobra.ExactArgs(1),
			Short: "Updates the checksums of a package",
			RunE: func(cmd *cobra.Command, args []string) error {
				pkgName := args[0]
				return runUpdateSums(pkgName)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "update-version <pkg> [new-version]",
			Short: "Updates the version of a package",
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
			Short: "Upgrades all packages",
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
		fmt.Println("Do you want to build the package now? [y/N]")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			return fmt.Errorf("aborting")
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
	for _, pkg := range listPackages() {
		fmt.Println("Updating " + pkg + "...")
		cmd := mkcmd(false, "git", "pull")
		cmd.Dir = mprDir(pkg)
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
	return nil
}

func runUpdateSums(pkgName string) error {
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

	cmd := exec.Command("makedeb", "-g")
	cmd.Dir = dir
	outputBytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("could not run makedeb -g: %s", err)
	}

	newSumsVarDecl := strings.TrimSpace(string(outputBytes)) // sha256sums=...
	sumName := strings.Split(newSumsVarDecl, "=")[0]         // e.g. "sha256sums"
	sumValue := strings.Split(newSumsVarDecl, "=")[1]        // e.g. "sha256sums"

	pkgbuild := NewPKGBUILD(dir)
	err = pkgbuild.updateVar(sumName, sumValue)
	if err != nil {
		return err
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

	return runUpdateSums(pkgName)
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
	files, err := ioutil.ReadDir(mprDir())
	if err != nil {
		log.Fatal(err)
	}

	var packages []string
	for _, file := range files {
		if file.IsDir() && file.Name() != "." {
			packages = append(packages, file.Name())
		}
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
	return spec
}
