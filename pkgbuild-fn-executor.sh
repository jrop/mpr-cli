#!/bin/bash
source ./PKGBUILD
if declare -f "$PKGBUILD_FN" > /dev/null
then
  # now call it:
  "$PKGBUILD_FN"
fi
