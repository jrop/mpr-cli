package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(func() error {
					pkgName := args[0]
					return runBuild(pkgName)
				})
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "check-stale",
			Short: "Checks for stale packages",
			Long:  `Checks for stale packages. A package is considered stale if it's version is behind repology's record.`,
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(runCheckStale)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "clone <package-url>",
			Short: "Clones a package",
			Long:  `Clones a package. This is equivalent to running "git clone" in the packages directory.`,
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(func() error {
					packageURL := args[0]
					return runClone(packageURL)
				})
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "each ...",
			Short: "Runs a command in each package's directory",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return fmt.Errorf("expected at least 1 argument, got 0")
				}

				runFallibleCommand(func() error {
					return runEach(args)
				})
				return nil
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "edit <package-name>",
			Short: "Edits a package's PKGBUILD",
			Long:  `Edits a package's PKGBUILD. This is equivalent to running "$EDITOR PKGBUILD" in the package's directory.`,
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(func() error {
					pkgName := args[0]
					return runEdit(pkgName)
				})
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
				Run: func(cmd *cobra.Command, args []string) {
					runFallibleCommand(func() error {
						packageURL := args[0]
						noConfirm, _ := cmd.Flags().GetBool("no-confirm")
						return runInstall(installArgs{
							packageURL: packageURL,
							confirm:    !noConfirm,
						})
					})
				},
			}
			cmd.Flags().BoolP("no-confirm", "c", false, "do not ask for confirmation")
			return cmd
		}())

		cmd.AddCommand(&cobra.Command{
			Use:   "list",
			Short: "Lists all packages",
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(runList)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "outdated",
			Short: "Lists all outdated packages",
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(runOutdated)
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "reinstall <pkg>",
			Short: "Reinstalls a package",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(func() error {
					pkgName := args[0]
					return runReinstall(pkgName)
				})
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "update [pkgs]",
			Short: "Updates all/specified packages (runs `git pull`)",
			Long:  `Updates all/specified packages. This is equivalent to running "git fetch" in each package's directory.`,
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(func() error {
					return runUpdate(args)
				})
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "update-version <pkg> [new-version]",
			Short: "Updates the version of a package in a PKGBUILD file",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) < 1 || len(args) > 2 {
					return fmt.Errorf("expected at 1 or 2 arguments, got %d", len(args))
				}

				runFallibleCommand(func() error {
					pkgName := args[0]
					newVersion := ""
					if len(args) == 2 {
						newVersion = args[1]
					}
					return runUpdateVersion(pkgName, newVersion)
				})
				return nil
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "uninstall <pkg>",
			Short: "Uninstalls a package",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(func() error {
					pkgName := args[0]
					return runUninstall(pkgName)
				})
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "info <pkg>",
			Args:  cobra.ExactArgs(1),
			Short: "Shows information about a package",
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(func() error {
					pkgName := args[0]
					return runPkgInfo(pkgName)
				})
			},
		})

		cmd.AddCommand(&cobra.Command{
			Use:   "upgrade [pkgs]",
			Short: "Installs newly available versions",
			Long:  `Upgrades all/selected packages. This is equivalent to running "makedeb ..." in each package's directory.`,
			Run: func(cmd *cobra.Command, args []string) {
				runFallibleCommand(func() error {
					confirm, _ := cmd.Flags().GetBool("confirm")
					return runUpgrade(upgradeArgs{
						packages: args,
						confirm:  confirm,
					})
				})
			},
		})

		// return the root command
		return cmd
	}()

	// run the command
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
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
		fmt.Fprintln(os.Stderr, "error:", err)
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
