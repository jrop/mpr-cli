#!/bin/bash

oldvars=$(set | grep -P "^[a-zA-Z0-9_]+=.*" | sort)
source ./PKGBUILD
newvars=$(set | grep -P "^[a-zA-Z0-9_]+=.*" | sort)
newvarnames=$(diff <(echo "$oldvars") <(echo "$newvars") | grep -P "^>" | sed -r "s/^> ([a-zA-Z0-9_]+)=.*$/\1/g" | sort)

for varName in $newvarnames; do
  case $varName in
    _|BASH_ARGC|PIPESTATUS|oldvars)
      continue
      ;;
  esac

  # if the variable specified by varName is an array, print the array,
  # duplicating the name for each element
  if [[ $(declare -p $varName 2>/dev/null) =~ "declare -a" ]]; then
    # Create a temporary array and copy the elements from the original array
    eval "temp_array=(\"\${$varName[@]}\")"
    for element in "${temp_array[@]}"; do
      echo "$varName=$element"
    done
  else
    echo "$varName=${!varName}"
  fi
done
