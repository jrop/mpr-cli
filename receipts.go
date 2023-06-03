package main

import (
	"io/ioutil"
	"os"
	"strings"
)

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
