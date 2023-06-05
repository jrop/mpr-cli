package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
)

func runFallibleCommand(f func() error) { // {{{
	if err := f(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
} // }}}

type installArgs struct {
	packageURL string
	confirm    bool
}

type upgradeArgs struct {
	packages []string
	confirm  bool
}

func runBuild(pkgName string) error { // {{{
	cmd := mkcmd(true, "makedeb")
	cmd.Dir = mprDir(pkgName)
	return cmd.Run()
} // }}}

func runCheckStale() error { // {{{
	packages := listPackages()
	var counter int64 = 0

	type pkgInfo struct {
		name    string
		version string
		newest  string
	}
	type pkgError struct {
		name string
		err  error
	}
	mux := sync.Mutex{}
	updatablePackages := make([]pkgInfo, 0)
	pkgWithErrors := make([]pkgError, 0)

	_setLine := func(line string) {
		line = fmt.Sprintf("(%d/%d) %s", counter, len(packages), line)
		mux.Lock()
		defer mux.Unlock()

		setLine(line)
	}

	err := doParallel(len(packages), 10, func(i int) error {
		defer atomic.AddInt64(&counter, 1)
		fullPkgName := packages[i]
		defer _setLine("Checked " + fullPkgName)

		addPackageError := func(err error) {
			mux.Lock()
			defer mux.Unlock()
			pkgWithErrors = append(pkgWithErrors, pkgError{
				name: fullPkgName,
				err:  err,
			})
		}

		pkgbuild := NewPKGBUILD(mprDir(fullPkgName))
		newestVersion, err := pkgbuild.getLatestRepologyPkgVersion()
		if err != nil {
			addPackageError(err)
			return nil
		}
		if newestVersion == "SKIP" {
			return nil
		}
		pkgver, err := pkgbuild.getSingleVariable("pkgver")
		if err != nil {
			addPackageError(fmt.Errorf("could not read pkgver variables"))
			return nil
		}

		// remove quotes/single quotes from start/end:
		pkgver = strings.Trim(pkgver, "\"")
		pkgver = strings.Trim(pkgver, "'")

		if newestVersion != pkgver {
			mux.Lock()
			updatablePackages = append(updatablePackages, pkgInfo{
				name:    fullPkgName,
				version: pkgver,
				newest:  newestVersion,
			})
			mux.Unlock()
		}

		return nil
	})
	fmt.Println()

	if err != nil {
		return err
	}
	if len(pkgWithErrors) > 0 {
		msg := ""
		for _, pkg := range pkgWithErrors {
			msg += fmt.Sprintf("- %s: %s\n", pkg.name, pkg.err)
		}
		return fmt.Errorf("some packages had errors:\n%s", msg)
	}

	for _, pkg := range updatablePackages {
		green := color.New(color.FgGreen).SprintFunc()
		red := color.New(color.FgRed).SprintFunc()
		fmt.Printf("%s: current=%s, latest=%s\n", pkg.name, red(pkg.version), green(pkg.newest))
	}

	return err
} // }}}

func runClone(packageURL string) error { // {{{
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
} // }}}

func runEach(args []string) error { // {{{
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
} // }}}

func runEdit(pkgName string) error { // {{{
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
} // }}}

func runInstall(args installArgs) error { // {{{
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
} // }}}

func runList() error { // {{{
	for _, pkg := range listPackages() {
		fmt.Println(pkg)
	}
	return nil
} // }}}

func runOutdated() error { // {{{
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
} // }}}

func runRecomputeSums(pkgName string) error { // {{{
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
} // }}}

func runReinstall(pkgName string) error { // {{{
	cmd := mkcmd(true, "makedeb", "-si")
	cmd.Dir = mprDir(pkgName)
	return cmd.Run()
} // }}}

func runUpdate(packagesToUpdate []string) error { // {{{
	packages := listPackages()
	if len(packagesToUpdate) > 0 {
		// validate that each package in packagesToUpdate exists:
		for _, pkg := range packagesToUpdate {
			installed := stringSliceContainsString(packages, pkg)
			if !installed {
				return fmt.Errorf("package not installed: %s", pkg)
			}
		}
		packages = packagesToUpdate
	}

	// create an atomic counter:
	var counter int64 = 0
	failedPackages := make([]string, 0)

	_setLine := func(line string) {
		line = fmt.Sprintf("(%d/%d) %s", counter, len(packages), line)
		setLine(line)
	}

	_setLine("Updating")
	err := doParallel(len(packages), 10, func(i int) error {
		pkg := packages[i]
		cmd := exec.Command("git", "pull")
		cmd.Dir = mprDir(pkg)
		// kill the command if it takes too long:
		timer := time.AfterFunc(10*time.Second, func() {
			if err := cmd.Process.Kill(); err != nil {
				panic(err)
			}
			_setLine(fmt.Sprintf("Killed: %s (took too long)", pkg))
		})
		_, err := cmd.Output()
		atomic.AddInt64(&counter, 1)
		timer.Stop()
		if err != nil {
			_setLine(fmt.Sprintf("Killed: %s (took too long)", pkg))
			failedPackages = append(failedPackages, pkg)
		}
		_setLine(fmt.Sprintf("Updated %s", pkg))

		return nil
	})
	fmt.Println()

	if err != nil {
		return err
	}

	if len(failedPackages) > 0 {
		return fmt.Errorf("mpr update failed for some packages: %s", strings.Join(failedPackages, ", "))
	}
	return nil
} // }}}

func runUpdateVersion(pkgName string, newVersion string) error { // {{{
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

	return runRecomputeSums(pkgName)
} // }}}

func runUninstall(pkgName string) error { // {{{
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
} // }}}

func runPkgInfo(pkgName string) error { // {{{
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
} // }}}

func runUpgrade(args upgradeArgs) error { // {{{
	packages := listPackages()
	if len(args.packages) > 0 {
		for _, pkg := range args.packages {
			installed := stringSliceContainsString(packages, pkg)
			if !installed {
				return fmt.Errorf("package not installed: %s", pkg)
			}
		}
		packages = args.packages
	}
	for _, pkg := range packages {
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
} // }}}
