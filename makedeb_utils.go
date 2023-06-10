package main

import (
	"regexp"
	"strings"
)

// The following is a utility to parse the output of `makedeb -g`, which is
// used to get the updated hashes of the sources. The output of `makedeb -g`
// looks like this:
//
//	$ makedeb -g
//	somevar1=somevalue1
//	somevar2=somevalue2
//	somevar3=somevalue3
//
// We need to get each new var declaration, e.g. "sha256sums=..." from the
// output, and there may be multiple. Each declaration starts at the beginning
// of a line, and is followed by an equals sign
func parseMakedebG(outputBytes []byte) map[string]string {
	varsToReplace := make(map[string]string)

	reg := regexp.MustCompile(`(?m)^[a-zA-Z0-9_]+=`)
	matches := reg.FindAllIndex(outputBytes, -1)
	for _, matchIndicies := range matches {
		a, b := matchIndicies[0], matchIndicies[1]

		matchStart := a
		matchEnd := -1 // this is the index of the beginning of the _next_ match
		nextMatch := reg.FindIndex(outputBytes[b:])
		if nextMatch == nil {
			matchEnd = len(outputBytes)
		} else {
			matchEnd = matchStart + nextMatch[1]
		}

		// Now we know enough to get the entire var declaration:
		varDecl := strings.TrimSpace(string(outputBytes[matchStart:matchEnd])) // sha256sums=...
		varName := strings.Split(varDecl, "=")[0]                              // e.g. "sha256sums"
		varValue := strings.Split(varDecl, "=")[1]                             // e.g. "sha256sums"

		// And now we know the variable name/value:
		varsToReplace[varName] = varValue
	}

	return varsToReplace
}
