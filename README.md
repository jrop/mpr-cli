# mpr-cli

> A CLI for the makedeb Package Repository

Put a compelling blurb here about why this is awesome...

## Installation

**Install Script**:

```sh
curl -sL https://raw.githubusercontent.com/jrop/mpr-cli/main/install.sh | bash
```

**Install from Git**:

```sh
git clone https://github.com/jrop/mpr-cli
cd mpr-cli
make # requires that Go is installed
sudo cp build/mpr /usr/local/bin/mpr
```

**Install from MPR**:
```
git clone https://mpr.makedeb.org/mpr # or mpr-bin
# OR: git clone https://mpr.makedeb.org/mpr-bin
cd mpr      # OR mpr-bin
makedeb -si # requires that makedeb is installed
```

**Install from MPR _using_ `mpr-cli`**:
```
mpr install mpr.makedeb.org/mpr
# OR: mpr install mpr.makedeb.org/mpr-bin
```

## Usage

At a high-level, these are the commands that you can run to manage your
packages:

```
Usage:
  mpr [flags]
  mpr [command]

Available Commands:
  build          Builds a package
  check-stale    Checks for stale packages
  clone          Clones a package
  completion     Generate the autocompletion script for the specified shell
  each           Runs a command in each package's directory
  edit           Edits a package's PKGBUILD
  help           Help about any command
  info           Shows information about a package
  install        Installs a package
  list           Lists all packages
  outdated       Lists all outdated packages
  recompute-sums Updates the checksums of a package
  reinstall      Reinstalls a package
  uninstall      Uninstalls a package
  update         Updates all/specified packages (runs `git pull`)
  update-version Updates the version of a package in a PKGBUILD file
  upgrade        Installs newly available versions

Flags:
  -h, --help      help for mpr
  -V, --version   print version information and exit

Use "mpr [command] --help" for more information about a command.
```

The package flow follows `apt` loosely:

1. Install new packages with `mpr install ...`
2. Fetch updates with `mpr update`
3. Install the latest available versions of packages with `mpr upgrade`

## `mpr install <package-url> [flags]`

The install command can take a few shorthand package "URLs":

- `mpr install mpr` - installs from https://mpr.makedeb.org/mpr
- `mpr install user/repo` - installs from https://github.com/user/repo
- ...all other forms _need_ to be valid URLs to a Git repository

## License (MIT)

MIT License

Copyright (c) 2023 Jonathan Apodaca <jrapodaca@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
