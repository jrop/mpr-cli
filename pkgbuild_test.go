package main

import (
	"testing"
)

func TestPKGBUILDUpdateVariable(t *testing.T) {
	// simple case
	pkgbuild, err := NewPKGBUILDFromContents("start=1\npkgname=foo\nend=2")
	if err != nil {
		t.Error(err)
	}
	err = pkgbuild.updateVar("pkgname", "bar")
	if err != nil {
		t.Error(err)
	}
	pkgname, err := pkgbuild.getSingleVariable("pkgname")
	if err != nil {
		t.Error(err)
	}
	if pkgname != "bar" {
		t.Errorf("Expected pkgname to be 'bar', got '%s'", pkgname)
	}

	// a longer case:
	err = pkgbuild.updateVar("pkgname", "somethingLongerThanFoo")
	if err != nil {
		t.Error(err)
	}
	pkgname, err = pkgbuild.getSingleVariable("pkgname")
	if err != nil {
		t.Error(err)
	}
	if pkgname != "somethingLongerThanFoo" {
		t.Errorf("Expected pkgname to be 'somethingLongerThanFoo', got '%s'", pkgname)
	}

	// replace a case surrounded by single quotes:
	pkgbuild, err = NewPKGBUILDFromContents("start=1\npkgname=\"foo\"\nend=2")
	if err != nil {
		t.Error(err)
	}
	err = pkgbuild.updateVar("pkgname", "\"bar\"")
	if err != nil {
		t.Error(err)
	}
	pkgname, err = pkgbuild.getSingleVariable("pkgname")
	if err != nil {
		t.Error(err)
	}
	if pkgname != "bar" {
		t.Errorf("Expected pkgname to be '\"bar\"', got '%s'", pkgname)
	}

	// replace a case surrounded by double quotes:
	pkgbuild, err = NewPKGBUILDFromContents("start=1\npkgname=\"foo\"\nend=2")
	if err != nil {
		t.Error(err)
	}
	err = pkgbuild.updateVar("pkgname", "\"bar\"")
	if err != nil {
		t.Error(err)
	}
	pkgname, err = pkgbuild.getSingleVariable("pkgname")
	if err != nil {
		t.Error(err)
	}
	if pkgname != "bar" {
		t.Errorf("Expected pkgname to be '\"bar\"', got '%s'", pkgname)
	}

	// replace case surrounded by parentheses
	pkgbuild, err = NewPKGBUILDFromContents("start=1\npkgname=(foo)\nend=2")
	if err != nil {
		t.Error(err)
	}
	err = pkgbuild.updateVar("pkgname", "(bar)")
	if err != nil {
		t.Error(err)
	}
	pkgname, err = pkgbuild.getSingleVariable("pkgname")
	if err != nil {
		t.Error(err)
	}
	if pkgname != "bar" {
		t.Errorf("Expected pkgname to be '(bar)', got '%s'", pkgname)
	}
}

func TestRealWorldUpdateVar(t *testing.T) {
	pkgbuildSource := `pkgname=nerd-fonts-fira-code-bin
repology_pkgname=fonts:nerd-fonts
pkgver=3.0.1
pkgrel=4
pkgdesc='Iconic font aggregator, collection, & patcher. 3,600+ icons, 50+ patched fonts: Hack, Source Code Pro, more. Glyph collections: Font Awesome, Material Design Icons, Octicons, & more'
arch=(amd64 i386)
depends=()
provides=(nerd-fonts-fira-code)
conflicts=(nerd-fonts-fira-code)
license=('Unknown')
url='https://nerdfonts.com/'
extensions=('zipman')`

	pkgbuild, err := NewPKGBUILDFromContents(pkgbuildSource)
	if err != nil {
		t.Error(err)
	}

	err = pkgbuild.updateVar("pkgver", "3.0.2")
	if err != nil {
		t.Error(err)
	}

	expectedPkgbuildSource := `pkgname=nerd-fonts-fira-code-bin
repology_pkgname=fonts:nerd-fonts
pkgver=3.0.2
pkgrel=4
pkgdesc='Iconic font aggregator, collection, & patcher. 3,600+ icons, 50+ patched fonts: Hack, Source Code Pro, more. Glyph collections: Font Awesome, Material Design Icons, Octicons, & more'
arch=(amd64 i386)
depends=()
provides=(nerd-fonts-fira-code)
conflicts=(nerd-fonts-fira-code)
license=('Unknown')
url='https://nerdfonts.com/'
extensions=('zipman')`

	if expectedPkgbuildSource != pkgbuild.contents {
		t.Errorf("Expected pkgbuild.contents to be:\n%s\n\nGot:\n%s\n", expectedPkgbuildSource, pkgbuild.contents)
	}
}
