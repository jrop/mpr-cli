package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
)

//go:embed pkgbuild-*.sh
var pkgbuildScripts embed.FS

type PKGBUILD struct {
	dirPath          string
	contents         string               // caches the contents of the PKGBUILD file
	contentsErr      error                // caches the error from reading the PKGBUILD file
	contentsOnce     sync.Once            // ensures that the contents are only read once
	allVariables     *map[string][]string // caches the variables from the PKGBUILD file
	allVariablesErr  error                // caches the error from getting the variables
	allVariablesOnce sync.Once            // ensures that the variables are only read once
}

type PKGBUILD_source struct {
	localName string
	remoteURL string
	hash      string
}

func NewPKGBUILD(dirPath string) *PKGBUILD {
	return &PKGBUILD{dirPath: dirPath, allVariables: nil}
}

// NewPKGBUILDFromContents creates a new PKGBUILD from the given contents. This
// is useful for when you want to create a PKGBUILD from scratch, or when you
// want to use this utility in an in-memory fashion.
func NewPKGBUILDFromContents(contents string) (*PKGBUILD, error) {
	dirPath, err := os.MkdirTemp(os.TempDir(), "pkgbuild-")
	if err != nil {
		return nil, err
	}

	p := PKGBUILD{
		dirPath:      dirPath,
		contents:     contents,
		contentsErr:  nil,
		contentsOnce: sync.Once{},
	}

	// mark the contents as read so that we don't try to read the file:
	p.contentsOnce.Do(func() {})
	return &p, nil
}

func (p *PKGBUILD) readContents() (string, error) { // {{{
	p.contentsOnce.Do(func() {
		contents, err := os.ReadFile(filepath.Join(p.dirPath, "PKGBUILD"))
		if err != nil {
			p.contents = ""
			p.contentsErr = err
			return
		}
		p.contents = string(contents)
		p.contentsErr = nil
	})
	return p.contents, p.contentsErr
} // }}}

func (p *PKGBUILD) writeContents(contents string) error { // {{{
	err := os.WriteFile(filepath.Join(p.dirPath, "PKGBUILD"), []byte(contents), 0644)
	if err != nil {
		return err
	}
	p.contents = contents
	p.contentsErr = nil

	// reset the allVariablesOnce:
	p.allVariablesOnce = sync.Once{}
	p.allVariables = nil
	p.allVariablesErr = nil

	return nil
} // }}}

// GetVariables returns a map of all of the variables in the PKGBUILD file. The
// mechanism for getting these variables assumes that the PKGBUILD script is
// idempotent, and that it can be run in a temporary directory without any
// side-effects. This is done by copying the PKGBUILD file to a temporary
// directory, and then running a script that prints out all of the variables
// that are set in the PKGBUILD file.
func (p *PKGBUILD) getVariables() (*map[string][]string, error) { // {{{
	p.allVariablesOnce.Do(func() {
		tmpDir, err := os.MkdirTemp(".", "tmp-pkgbuild")
		defer os.RemoveAll(tmpDir)

		contents, err := p.readContents()
		if err != nil {
			p.allVariablesErr = err
			return
		}
		err = os.WriteFile(filepath.Join(tmpDir, "PKGBUILD"), []byte(contents), 0644)
		if err != nil {
			p.allVariablesErr = err
			return
		}

		// get the pkgbuild-var-printer.sh from the embeded files
		script, err := pkgbuildScripts.ReadFile("pkgbuild-var-printer.sh")
		if err != nil {
			p.allVariablesErr = err
			return
		}

		// write the script to the file
		err = os.WriteFile(filepath.Join(tmpDir, "pkgbuild-var-printer.sh"), script, 0755)
		if err != nil {
			p.allVariablesErr = err
			return
		}

		// run the script
		cmd := exec.Command("bash", "pkgbuild-var-printer.sh")
		cmd.Dir = tmpDir
		out, err := cmd.Output()
		if err != nil {
			p.allVariablesErr = err
			return
		}

		// now for each line in the output, split on `=` and add to the map
		vars := make(map[string][]string)
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}

			parts := strings.SplitN(string(line), "=", 2)
			if len(parts) != 2 {
				p.allVariablesErr = fmt.Errorf("invalid line in output: %s", string(line))
				return
			}
			if _, ok := vars[parts[0]]; !ok {
				vars[parts[0]] = make([]string, 0)
			}
			vars[parts[0]] = append(vars[parts[0]], parts[1])
		}

		p.allVariables = &vars
		p.allVariablesErr = nil
	})

	return p.allVariables, p.allVariablesErr
} // }}}

func (p *PKGBUILD) getVariablesMerged() (*map[string][]string, error) { // {{{
	varsAddr, err := p.getVariables()
	if err != nil {
		return nil, err
	}
	vars := *varsAddr

	// Do variable merging to make other operations more simple. That is, if a
	// variable named `foo_<ARCH>` exists, then merge it with the `foo`
	// variable. This will make it easier to get the source variable, for
	// example. We will use runtime.GOARCH to get the architecture of the
	// current system.
	arch := runtime.GOARCH
	for name, val := range vars {
		if !strings.HasSuffix(name, "_"+arch) {
			continue
		}

		baseName := strings.TrimSuffix(name, "_"+arch)

		// if vars[baseName] doesn't exist, then create it:
		if _, ok := vars[baseName]; !ok {
			vars[baseName] = make([]string, 0)
		}

		// overwrite the baseName variable with the arch-specific one:
		vars[baseName] = append(vars[baseName], val...)
	}

	return &vars, nil
} // }}}

func (p *PKGBUILD) getVariable(name string) ([]string, error) { // {{{
	vars, err := p.getVariables()
	if err != nil {
		return nil, err
	}

	if val, ok := (*vars)[name]; ok {
		return val, nil
	}

	return nil, fmt.Errorf("variable %s not found", name)
} // }}}

func (p *PKGBUILD) getSingleVariable(name string) (string, error) { // {{{
	val, err := p.getVariable(name)
	if err != nil {
		return "", err
	}

	if len(val) != 1 {
		return "", fmt.Errorf("variable %s has %d values", name, len(val))
	}

	return val[0], nil
} // }}}

func (p *PKGBUILD) updateVar(varName string, newValue string) error { // {{{
	source, err := p.readContents()
	if err != nil {
		return err
	}

	// find the variable
	varPrefix := varName + "="
	varPrefixStart := strings.Index(source, varPrefix)
	if varPrefixStart == -1 {
		return fmt.Errorf("variable %s not found", varName)
	}

	varPrefixEnd := varPrefixStart + len(varPrefix)
	start := varPrefixEnd

	if source[varPrefixStart:varPrefixEnd] != varPrefix {
		panic(fmt.Sprintf("this should never happen: got %s", source[varPrefixStart:varPrefixEnd]))
	}

	// 1. if next char is ', find the matching ' and that is the end
	// 2. if next char is ", find the matching " and that is the end
	// 2. if next char is (, find the matching ) and that is the end
	// 3. the end is the first whitespace after the = sign

	varEnd := start
	if source[start] == '\'' {
		// find the matching ', skipping over escape sequences:
		for {
			varEnd = strings.Index(source[varEnd+1:], "'") + varEnd + 2
			if source[varEnd-1] != '\\' {
				break
			}
			varEnd++
		}
	} else if source[start] == '"' {
		// find the matching ", skipping over escape sequences:
		for {
			varEnd = strings.Index(source[varEnd+1:], "\"") + varEnd + 2
			if source[varEnd-1] != '\\' {
				break
			}
			varEnd++
		}
	} else if source[start] == '(' {
		// find the matching ):
		varEnd = strings.Index(source[start:], ")") + start + 1
	} else {
		idx := strings.IndexFunc(source[start:], unicode.IsSpace)
		if idx == -1 {
			idx = len(source[start:])
		}
		varEnd = start + idx
	}

	// replace the variable:
	source = source[:start] + newValue + source[varEnd:]
	err = p.writeContents(source)
	if err != nil {
		return err
	}

	return nil
} // }}}

func (p *PKGBUILD) getHashes() ([]string, error) { // {{{
	// Return the first of the following variables that exists:
	// cksums, md5sums, sha1sums, sha224sums, sha256sums, sha384sums, sha512sums, b2sums
	for _, name := range []string{"cksums", "md5sums", "sha1sums", "sha224sums", "sha256sums", "sha384sums", "sha512sums", "b2sums"} {
		if val, err := p.getVariable(name); err == nil {
			return val, nil
		}
	}

	// return an empty slice if none of the above variables exist
	emptyHashes := make([]string, 0)
	return emptyHashes, nil
} // }}}

func (p *PKGBUILD) getSources() ([]PKGBUILD_source, error) { // {{{
	hashesVar, err := p.getHashes()
	if err != nil {
		return nil, err
	}
	sourceVar, err := p.getVariable("source")
	if err != nil {
		return nil, err
	}
	if len(hashesVar) != len(sourceVar) {
		return nil, fmt.Errorf("source and hashes variables have different lengths")
	}

	sources := make([]PKGBUILD_source, 0)
	for idx, sourceSpec := range sourceVar {
		sourceInfo := PKGBUILD_source{
			localName: filepath.Base(sourceSpec),
			remoteURL: sourceSpec,
			hash:      hashesVar[idx],
		}

		if strings.Contains(sourceSpec, "::") {
			parts := strings.SplitN(sourceSpec, "::", 2)
			sourceInfo.localName = parts[0]
			sourceInfo.remoteURL = parts[1]
		}

		sources = append(sources, sourceInfo)
	}

	return sources, nil
} // }}}

func (p *PKGBUILD) getRepologyPkgname() (string, error) { // {{{
	val, err := p.getVariable("repology_pkgname")
	if err == nil {
		return val[0], nil
	}

	val, err = p.getVariable("pkgname")
	if err == nil {
		val[0] = strings.TrimSuffix(val[0], "-bin")
		val[0] = strings.TrimSuffix(val[0], "-git")
		return val[0], nil
	}

	return "", fmt.Errorf("repology_pkgname or pkgname not found")
} // }}}

func (p *PKGBUILD) getLatestRepologyPkgVersion() (string, error) { // {{{
	pkgname, err := p.getRepologyPkgname()
	if err != nil {
		return "", err
	}
	if pkgname == "SKIP" {
		return "SKIP", nil
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get("https://repology.org/api/v1/project/" + pkgname)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", err
	}
	var newData []map[string]interface{}
	for _, entry := range data {
		if entry["status"] == "newest" {
			newData = append(newData, entry)
		}
	}

	var versions []string
	for _, entry := range newData {
		versions = append(versions, entry["version"].(string))
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("could not find any versions for package %s", pkgname)
	}
	return versions[0], nil
} // }}}

func (p *PKGBUILD) executeFunction(fnName string) error { // {{{
	tmpScript, err := os.CreateTemp(p.dirPath, "pkgbuild-fn-executor.sh")
	if err != nil {
		return err
	}
	defer os.Remove(tmpScript.Name())

	// get the pkgbuild-fn-executor.sh from the embeded files
	script, err := pkgbuildScripts.ReadFile("pkgbuild-fn-executor.sh")
	if err != nil {
		return err
	}

	// write the script to the file
	_, err = tmpScript.Write(script)
	if err != nil {
		return err
	}

	// run the script
	cmd := exec.Command("bash", tmpScript.Name())
	cmd.Dir = p.dirPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("PKGBUILD_FN=%s", fnName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
} // }}}

func (p *PKGBUILD) downloadSources() ([]PKGBUILD_source, error) { // {{{
	mkerr := func(err error) ([]PKGBUILD_source, error) {
		return make([]PKGBUILD_source, 0), err
	}
	sources, err := p.getSources()
	if err != nil {
		return mkerr(err)
	}

	for _, source := range sources {
		// parse the URL scheme of remoteURL so we know how to download it
		parsedRemoteURL, err := url.Parse(source.remoteURL)
		if err != nil {
			return mkerr(err)
		}

		switch parsedRemoteURL.Scheme {
		case "http", "https":
			// TODO: convert to this pure Go in the future: {{{
			// fmt.Printf("Downloading %s\n", source.remoteURL)
			// resp, err := http.Get(source.remoteURL)
			// if err != nil {
			// 	return mkerr(err)
			// }
			// defer resp.Body.Close()

			// totalLength := resp.ContentLength

			// localFile, err := os.Create(filepath.Join(p.dirPath, source.localName))
			// if err != nil {
			// 	return mkerr(err)
			// }
			// defer localFile.Close()

			// _, err = io.Copy(
			// 	localFile,
			// 	io.TeeReader(
			// 		resp.Body,
			// 		newProgressReader(
			// 			totalLength,
			// 			func(progress int64, totalLength int64) {
			// 				// TODO: print a nice progress bar:
			// 			},
			// 		),
			// 	),
			// )
			// if err != nil {
			// 	return mkerr(err)
			// }
			// }}}
			cmd := exec.Command("curl", "-L", "-o", source.localName, source.remoteURL)
			cmd.Dir = p.dirPath
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			err = cmd.Run()
			if err != nil {
				return mkerr(err)
			}

			// TODO: verify the hash of the downloaded file

		case "git", "git+ssh":
			// TODO

			// Q: How to handle localName?

			// If you want to check out a specific commit, you would put the commit
			// hash in the #commit=<hash> fragment of the source URL, or use a
			// #tag=<tag> or #branch=<branch> fragment. Here is an example:
		}
	}

	return sources, nil
} // }}}
