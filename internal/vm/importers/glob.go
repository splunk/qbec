package importers

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/pkg/errors"
)

const (
	globParamDirs           = "dirs"
	globParamVerb           = "verb"
	globParamStripExtension = "strip-extension"
)

var supportedParams = map[string]bool{
	globParamDirs:           true,
	globParamVerb:           true,
	globParamStripExtension: true,
}

var supportedParamsDisplay string

func init() {
	var ret []string
	for k := range supportedParams {
		ret = append(ret, k)
	}
	sort.Strings(ret)
	supportedParamsDisplay = strings.Join(ret, ", ")
}

// globImportParams are additional parameters for the glob import expressed as URI query params.
type globImportParams struct {
	verb           string // one of 'import' or 'importstr' - the verb to use to import files found
	dirs           int    // number of directories levels to retain in the returned object (default: all)
	stripExtension bool   // whether extensions should be stripped from the base name of the file in the map key
}

// keyFor returns the key to be used for the supplied file based on whether directory levels should be limited
// and/ or extensions stripped.
func (p globImportParams) keyFor(file string) string {
	// short-circuit simple case
	if p.dirs < 0 && !p.stripExtension {
		return file
	}
	elements := strings.Split(file, string(filepath.Separator))
	name := elements[len(elements)-1]
	dirs := elements[0 : len(elements)-1]
	if p.dirs >= 0 {
		if len(dirs) > p.dirs {
			dirs = dirs[len(dirs)-p.dirs:]
		}
	}
	if p.stripExtension {
		pos := strings.LastIndex(name, ".")
		if pos > 0 {
			name = name[:pos]
		}
	}
	finalElements := append(dirs, name)
	return filepath.Join(finalElements...)
}

func (p globImportParams) String() string {
	return fmt.Sprintf("?dirs=%d&strip-extension=%t&verb=%s", p.dirs, p.stripExtension, p.verb)
}

func newGlobParams(query url.Values) (params globImportParams, err error) {
	for k := range query {
		if !supportedParams[k] {
			return params, fmt.Errorf("invalid query parameter '%s', allowed values are: %s", k, supportedParamsDisplay)
		}
	}
	verb := query.Get(globParamVerb)
	if verb == "" {
		verb = "import"
	}
	if verb != "import" && verb != "importstr" {
		return params, fmt.Errorf("'%s' parameter for glob import must be one of 'import' or 'importstr', found '%s'", globParamVerb, verb)
	}
	dirs := -1
	dirsStr := query.Get(globParamDirs)
	if dirsStr != "" {
		l, err := strconv.Atoi(dirsStr)
		if err != nil {
			return params, fmt.Errorf("invalid value '%s' for '%s' parameter, %v", dirsStr, globParamDirs, err)
		}
		if l < 0 {
			l = -1
		}
		dirs = l
	}
	stripExtension := false
	stripStr := query.Get(globParamStripExtension)
	if stripStr != "" {
		switch stripStr {
		case "false":
		case "true":
			stripExtension = true
		default:
			return params, fmt.Errorf("invalid value '%s' for '%s' parameter, must be 'true' or 'false'", stripStr, globParamStripExtension)
		}
	}
	return globImportParams{
		verb:           verb,
		dirs:           dirs,
		stripExtension: stripExtension,
	}, nil
}

// globEntry is a cache entry for a glob path resolved from the current working directory.
type globEntry struct {
	contents jsonnet.Contents
	foundAt  string
	err      error
}

// GlobImporter provides facilities to import a bag of files using a glob pattern. Note that it will NOT
// honor any library paths and must be exactly resolved from the caller's location.
//
// The input to the glob importer is a URI having a scheme "glob" that has a file pattern in its path
// that is resolved using Go's glob functionality. The return value is an object that is keyed by file names
// with values importing the contents of the file.
//
// That is, given the following directory structure:
//
// 		lib
//			- a.json
//			- b.json
//		caller
//			- c.libsonnet
//
// where c.libsonnet has the following contents
//
//		import 'glob:../lib/*.json'
//
// evaluating `c.libsonnet` will return jsonnet code of the following form:
//
//		{
//			'../lib/a.json': import '../lib/a.json',
//			'../lib/b.json': import '../lib/b.json',
//		}
//
// The returned keys of the object and the import mechanism can be customized using query parameters to the glob URI. The parameters supported are:
//
// dirs: (number, default=-1) - the number of directory paths to include in the keys of the output. A value of 0 will only have the file name. A negative value preserves the full path.
//
// strip-extension: (bool, default=false) when `true`, will remove the extension of the file from the key of the returned object
//
// verb: (string, one of `import` or `importstr`, default=`import`) - the verb to use for importing inner files
//
// So, using the previous example if the jsonnet code were of the form:
//		import 'glob:../lib/*.json?dirs=0&strip-extension=true&verb=importstr'
//
// this will return jsonnet code of the following form:
//
//		{
//			'a': importstr '../lib/a.json',
//			'b': importstr '../lib/b.json',
//		}
//
// While glob patterns like `*` can be used as-is in the URI, the glob character `?` needs to be escaped as `%3F` so that
// it is not treated as a query parameter separator.
//
type GlobImporter struct {
	cache map[string]*globEntry
}

// NewGlobImporter creates a glob importer.
func NewGlobImporter() *GlobImporter {
	return &GlobImporter{cache: map[string]*globEntry{}}
}

func (g *GlobImporter) cacheKey(globPath string, p globImportParams) string {
	return fmt.Sprintf("%s%s", globPath, p.String())
}

// getEntry returns an entry from the cache or nil, if not found.
func (g *GlobImporter) getEntry(globPath string, p globImportParams) *globEntry {
	key := g.cacheKey(globPath, p)
	ret := g.cache[key]
	return ret
}

// setEntry sets the cache entry for the supplied path.
func (g *GlobImporter) setEntry(globPath string, p globImportParams, e *globEntry) {
	key := g.cacheKey(globPath, p)
	g.cache[key] = e
}

// CanProcess implements the interface method, returning true for paths that start with the string "glob:"
func (g *GlobImporter) CanProcess(path string) bool {
	return strings.HasPrefix(path, "glob:")
}

// Import implements the interface method.
func (g *GlobImporter) Import(importedFrom, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {
	// baseDir is the directory from which things are relatively imported
	baseDir, _ := filepath.Split(importedFrom)

	// parse it as a URI instead of stripping leading `glob:` so we get query parsing for free
	u, err := url.Parse(importedPath)
	if err != nil {
		return contents, foundAt, err
	}

	// if the opaque path is blank, someone most likely did import glob://rel-path or glob:/abs-path, dont' allow this
	relativeGlob := u.Opaque
	if relativeGlob == "" {
		return contents, foundAt, fmt.Errorf("unable to parse URI %q, ensure you did not use '/' or '//' after 'glob:'", importedPath)
	}
	relativeGlob, err = url.PathUnescape(relativeGlob)
	if err != nil {
		return contents, foundAt, fmt.Errorf("unable to unescape URI %q, %v", importedPath, err)
	}

	params, err := newGlobParams(u.Query())
	if err != nil {
		return contents, foundAt, errors.Wrap(err, importedPath)
	}

	// globPath is the glob path relative to the working directory
	globPath := filepath.Clean(filepath.Join(baseDir, relativeGlob))
	r := g.getEntry(globPath, params)
	if r != nil {
		return r.contents, r.foundAt, r.err
	}

	// once we have successfully gotten a glob path, we can store results in the cache
	defer func() {
		g.setEntry(globPath, params, &globEntry{
			contents: contents,
			foundAt:  foundAt,
			err:      err,
		})
	}()

	matches, err := filepath.Glob(globPath)
	if err != nil {
		return contents, foundAt, fmt.Errorf("unable to expand glob %q, %v", globPath, err)
	}

	// convert matches to be relative to our baseDir
	var relativeMatches []string
	for _, m := range matches {
		rel, err := filepath.Rel(baseDir, m)
		if err != nil {
			return contents, globPath, fmt.Errorf("could not resolve %s from %s", m, importedFrom)
		}
		relativeMatches = append(relativeMatches, rel)
	}

	// ensure consistent order (although this is probably not strictly required, makes it human friendly)
	sort.Strings(relativeMatches)

	seen := map[string]string{}

	var out bytes.Buffer
	out.WriteString("{\n")
	for _, file := range relativeMatches {
		key := params.keyFor(file)
		oldFile, ok := seen[key]
		if ok {
			return contents, foundAt, fmt.Errorf("%s: at least 2 files '%s' and '%s' map to the same key '%s'", importedPath, oldFile, file, key)
		}
		seen[key] = file
		out.WriteString("\t")
		_, _ = fmt.Fprintf(&out, `'%s': %s '%s',`, key, params.verb, file)
		out.WriteString("\n")
	}
	out.WriteString("}")
	hash := sha256.New()
	hash.Write(out.Bytes())
	sum := base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
	output := out.String()

	return jsonnet.MakeContents(output),
		filepath.Join(baseDir, fmt.Sprintf("%s.glob", sum)),
		nil
}
