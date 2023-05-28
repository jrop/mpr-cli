# mpr-cli

> A CLI for the makedeb Package Repository

Put a compelling blurb here about why this is awesome...

## Installation

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

## Workflow Case Study 1: Installing/Updating Packages

TODO...

## Workflow Case Study 2: Package Maintainer

TODO...

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
