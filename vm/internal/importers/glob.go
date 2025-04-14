/*
   Copyright 2021 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package importers

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/google/go-jsonnet"
)

// globCacheKey is the key to use for the importer cache. Two entries are equivalent if
// they resolve to the same set of files, have the same relative path to access them, and
// the same inner verb for access.
type globCacheKey struct {
	verb     string // the inner verb used for import
	resolved string // the glob pattern as resolved from the current working directory
	relative string // the glob pattern as specified by the user
}

var separator = []byte{0}

// file returns a virtual file name as represented by this cache key, relative to the supplied base directory.
func (c globCacheKey) file(baseDir string) string {
	h := sha256.New()
	h.Write([]byte(c.verb))
	h.Write(separator)
	h.Write([]byte(c.resolved))
	h.Write(separator)
	h.Write([]byte(c.relative))
	baseName := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	fileName := fmt.Sprintf("%s-%s.glob", baseName, c.verb)
	return filepath.Join(baseDir, fileName)
}

// globEntry is a cache entry for a glob path resolved from the current working directory.
type globEntry struct {
	contents jsonnet.Contents
	foundAt  string
	err      error
}

// GlobImporter provides facilities to import a bag of files using a glob pattern. Note that it will NOT
// honor any library paths and must be exactly resolved from the caller's location. It is initialized with
// a verb that configures how the inner imports are done (i.e. `import` or `importstr`) and it processes
// paths that start with `glob-{verb}:`
//
// After the marker prefix is stripped, the input is treated as a file pattern that is resolved using Go's glob functionality.
// The return value is an object that is keyed by file names relative to the import location with values
// importing the contents of the file.
//
// That is, given the following directory structure:
//
//	lib
//		- a.json
//		- b.json
//	caller
//		- c.libsonnet
//
// where c.libsonnet has the following contents
//
//	import 'glob-import:../lib/*.json'
//
// evaluating `c.libsonnet` will return jsonnet code of the following form:
//
//	{
//		'../lib/a.json': import '../lib/a.json',
//		'../lib/b.json': import '../lib/b.json',
//	}
type GlobImporter struct {
	innerVerb string
	prefix    string
	cache     map[globCacheKey]*globEntry
}

// NewGlobImporter creates a glob importer.
func NewGlobImporter(innerVerb string) *GlobImporter {
	if !(innerVerb == "import" || innerVerb == "importstr") {
		panic("invalid inner verb " + innerVerb + " for glob importer")
	}
	return &GlobImporter{
		innerVerb: innerVerb,
		prefix:    fmt.Sprintf("glob-%s:", innerVerb),
		cache:     map[globCacheKey]*globEntry{},
	}
}

func (g *GlobImporter) cacheKey(resolved, relative string) globCacheKey {
	return globCacheKey{
		verb:     g.innerVerb,
		resolved: resolved,
		relative: relative,
	}
}

// getEntry returns an entry from the cache or nil, if not found.
func (g *GlobImporter) getEntry(resolved, relative string) *globEntry {
	ret := g.cache[g.cacheKey(resolved, relative)]
	return ret
}

// setEntry sets the cache entry for the supplied path.
func (g *GlobImporter) setEntry(resolved, relative string, e *globEntry) {
	g.cache[g.cacheKey(resolved, relative)] = e
}

// CanProcess implements the interface method, returning true for paths that start with the string "glob:"
func (g *GlobImporter) CanProcess(path string) bool {
	return strings.HasPrefix(path, g.prefix)
}

// Import implements the interface method.
func (g *GlobImporter) Import(importedFrom, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {
	// baseDir is the directory from which things are relatively imported
	baseDir, _ := path.Split(importedFrom)

	relativeGlob := strings.TrimPrefix(importedPath, g.prefix)

	if strings.HasPrefix(relativeGlob, "/") {
		return contents, foundAt, fmt.Errorf("invalid glob pattern '%s', cannot be absolute", relativeGlob)
	}

	baseDir = filepath.FromSlash(baseDir)
	relativeGlob = filepath.FromSlash(relativeGlob)

	// globPath is the glob path relative to the working directory
	globPath := filepath.Clean(filepath.Join(baseDir, relativeGlob))
	r := g.getEntry(globPath, relativeGlob)
	if r != nil {
		return r.contents, r.foundAt, r.err
	}

	// once we have successfully gotten a glob path, we can store results in the cache
	defer func() {
		g.setEntry(globPath, relativeGlob, &globEntry{
			contents: contents,
			foundAt:  foundAt,
			err:      err,
		})
	}()

	fsDir, pat := doublestar.SplitPattern(filepath.ToSlash(globPath))
	matches, err := doublestar.Glob(os.DirFS(fsDir), pat)
	if err != nil {
		return contents, foundAt, fmt.Errorf("unable to expand glob %q, %v", globPath, err)
	}

	// convert matches to be relative to our baseDir
	var relativeMatches []string
	for _, m := range matches {
		m = path.Join(fsDir, m)
		rel, err := filepath.Rel(baseDir, m)
		if err != nil {
			return contents, globPath, fmt.Errorf("could not resolve %s from %s", m, importedFrom)
		}
		relativeMatches = append(relativeMatches, rel)
	}

	// ensure consistent order (not strictly required, makes it human friendly)
	sort.Strings(relativeMatches)

	var out bytes.Buffer
	out.WriteString("{\n")
	for _, file := range relativeMatches {
		file = filepath.ToSlash(file)
		out.WriteString("\t")
		_, _ = fmt.Fprintf(&out, `'%s': %s '%s',`, file, g.innerVerb, file)
		out.WriteString("\n")
	}
	out.WriteString("}")
	k := g.cacheKey(globPath, relativeGlob)
	output := out.String()

	return jsonnet.MakeContents(output),
		filepath.ToSlash(k.file(baseDir)),
		nil
}
