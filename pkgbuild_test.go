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
