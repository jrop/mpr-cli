#!/bin/bash
set -e

# Install makedeb, if it is not already installed:
if ! command -v makedeb &> /dev/null
then
  bash <(wget -qO - 'https://shlink.makedeb.org/install')
fi

mkdir -p ~/.cache/mpr-packages/
cd ~/.cache/mpr-packages/

if [ -d 'mpr-bin' ]; then
  echo 'mpr-bin already installed'
  exit 1
fi

git clone https://mpr.makedeb.org/mpr-bin.git
cd mpr-bin
makedeb -si
git rev-parse HEAD > .git/makedeb-install-receipt
cd -
