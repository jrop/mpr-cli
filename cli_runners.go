package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
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

type updateArgs struct {
	packagesToUpdate []string
	upgrade          bool
	confirm          bool
}

type upgradeArgs struct {
	packages []string
	confirm  bool
}

func runBuild(pkgName string) error { // {{{
	fmt.Printf("=> building %s\n", pkgName)
	cmd := mkcmd(true, "makedeb")
	cmd.Dir = mprDir(pkgName)
	return cmd.Run()
} // }}}

func runCheckStale() error { // {{{
	packages, err := listPackages()
	if err != nil {
		return err
	}
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

	for i := range packages {
		fullPkgName := packages[i]

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
			continue
		}
		if newestVersion == "SKIP" {
			continue
		}
		pkgver, err := pkgbuild.getSingleVariable("pkgver")
		if err != nil {
			addPackageError(fmt.Errorf("could not read pkgver variables"))
			continue
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

		time.Sleep(1100 * time.Millisecond)
		atomic.AddInt64(&counter, 1)
		_setLine("Checked " + fullPkgName)
	}
	fmt.Println()

	if len(pkgWithErrors) > 0 {
		msg := ""
		for _, pkg := range pkgWithErrors {
			msg += fmt.Sprintf("- %s: %s\n", pkg.name, pkg.err)
		}
		err = fmt.Errorf("some packages had errors:\n%s", msg)
	}

	for _, pkg := range updatablePackages {
		green := color.New(color.FgGreen).SprintFunc()
		red := color.New(color.FgRed).SprintFunc()
		fmt.Printf("%s: current=%s, latest=%s\n", pkg.name, red(pkg.version), green(pkg.newest))
	}

	return err
} // }}}

func runClean(packages []string) error { // {{{
	availablePkgs, err := listPackages()
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		packages = availablePkgs
	}

	for _, pkg := range packages {
		if !stringSliceContainsString(availablePkgs, pkg) {
			fmt.Printf("package %s does not exist\n", pkg)
			continue
		}

		fmt.Printf("=> cleaning %s\n", pkg)
		cmd := mkcmd(true, "git", "clean", "-fdx")
		cmd.Dir = mprDir(pkg)
		if err = cmd.Run(); err != nil {
			return err
		}
	}

	return nil
} // }}}

func runClone(packageURL string) error { // {{{
	url := getPackageURL(packageURL)
	pkg := filepath.Base(url)
	if strings.Contains(pkg, ":") {
		pkg = strings.Split(pkg, ":")[1]
	}
	pkg = strings.TrimSuffix(pkg, ".git")

	packages, err := listPackages()
	if err != nil {
		return err
	}
	for _, existingPkg := range packages {
		if existingPkg == pkg {
			fmt.Println("package already exists")
			os.Exit(1)
		}
	}
	fmt.Printf("=> cloning %s\n", pkg)
	cmd := mkcmd(true, "git", "clone", url, pkg)
	if err := cmd.Run(); err != nil {
		// clean up a botched clone:
		os.RemoveAll(mprDir(pkg))
		return err
	}
	return nil
} // }}}

func runEach(args []string) error { // {{{
	packages, err := listPackages()
	if err != nil {
		return err
	}
	for _, pkg := range packages {
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

	fmt.Printf("=> installing %s\n", pkg)
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
	packages, err := listPackages()
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		fmt.Println(pkg)
	}
	return nil
} // }}}

func runOutdated() error { // {{{
	var outdatedPkgs []string
	packages, err := listPackages()
	if err != nil {
		return err
	}
	for _, pkg := range packages {
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

func runRecomputeSums(pkgName string, edit bool) error { // {{{
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

	varsToReplace := parseMakedebG(outputBytes)
	pkgbuild := NewPKGBUILD(dir)
	for varName, varValue := range varsToReplace {
		err = pkgbuild.updateVar(varName, varValue)
		if err != nil {
			return err
		}
	}

	cmd = exec.Command("makedeb", "--print-srcinfo")
	cmd.Dir = dir
	outputBytes, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("could not run makedeb --print-srcinfo: %s", err)
	}

	os.WriteFile(path.Join(dir, ".SRCINFO"), outputBytes, 0)

	if edit {
		return runEdit(pkgName)
	}

	return nil
} // }}}

func runReinstall(pkgName string) error { // {{{
	fmt.Printf("=> reinstalling %s\n", pkgName)
	cmd := mkcmd(true, "makedeb", "-si")
	cmd.Dir = mprDir(pkgName)
	return cmd.Run()
} // }}}

func runUpdate(args updateArgs) error { // {{{
	packages, err := listPackages()
	if err != nil {
		return err
	}
	if len(args.packagesToUpdate) > 0 {
		// validate that each package in packagesToUpdate exists:
		for _, pkg := range args.packagesToUpdate {
			installed := stringSliceContainsString(packages, pkg)
			if !installed {
				return fmt.Errorf("package not installed: %s", pkg)
			}
		}
		packages = args.packagesToUpdate
	}

	// create an atomic counter:
	var counter int64 = 0
	failedPackages := make([]string, 0)

	_setLine := func(line string) {
		line = fmt.Sprintf("(%d/%d) %s", counter, len(packages), line)
		setLine(line)
	}

	_setLine("Updating")
	err = doParallel(len(packages), 10, func(i int) error {
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

	if args.upgrade {
		return runUpgrade(upgradeArgs{
			packages: args.packagesToUpdate,
			confirm:  args.confirm,
		})
	} else {
		fmt.Println("Checking for outdated packages...")
		return runOutdated()
	}
} // }}}

func runUpdateVersion(pkgName string, newVersion string, edit bool) error { // {{{
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

	return runRecomputeSums(pkgName, edit)
} // }}}

func runUninstall(pkgName string) error { // {{{
	installedPkgs, err := listPackages()
	if err != nil {
		return err
	}
	if !stringSliceContainsString(installedPkgs, pkgName) {
		return fmt.Errorf("package %s is not installed", pkgName)
	}

	// uninstall the package:
	fmt.Printf("=> uninstalling %s\n", pkgName)
	cmd := mkcmd(true, "sudo", "apt-get", "remove", pkgName)
	if err = cmd.Run(); err != nil {
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
	packages, err := listPackages()
	if err != nil {
		return err
	}
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

		fmt.Printf("=> upgrading %s\n", pkg)
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
