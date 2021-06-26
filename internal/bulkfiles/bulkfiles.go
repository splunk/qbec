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

// Package bulkfiles provides facilities to process files in bulk by walking down directory trees.
package bulkfiles

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// Processor indicates whether a file matches for processing, and allows some arbitrary processing on it
// It is only ever given files, never directories for processing.
type Processor interface {
	// Matches returns true if the specified path should be processed. The `userSpecified` argument
	// indicates that the file was explicitly passed in by the user.
	Matches(path string, file fs.FileInfo, userSpecified bool) bool
	// Process processes the specified file and returns an error in case of processing errors.
	// When processing is set to continue on errors, it is the function's responsibility to print
	// a detailed error. The bulk processor will only return an aggregate error containing stats about
	// the number of errors.
	Process(path string, file fs.FileInfo) error
}

// Options are options for processing.
type Options struct {
	ContinueOnError bool // continue processing other files in the face of errors returned by the processor
}

// shouldProcess returns true if the file should be processed. Currently always returns true but is pending
// some exclusion options that will make it meaningful later on.
func (o *Options) shouldProcess(path string, entry fs.FileInfo) (bool, error) {
	return true, nil
}

type entry struct {
	path string
	info os.FileInfo
}

type errorCount struct {
	numErrors int
}

func (e *errorCount) reportError(err error) error {
	if err == nil {
		return nil
	}
	e.numErrors++
	return err
}

func (e *errorCount) Error() error {
	switch e.numErrors {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("1 error encountered")
	default:
		return fmt.Errorf("%d errors encountered", e.numErrors)
	}
}

// Process processes the specified files using the specified options and processor.
// It returns an error on filesystem errors or if the processor reports an error
// for one or more files passed to it. Specified files must be valid for processing
// to start.
func Process(specified []string, opts Options, processor Processor) error {
	var entries []entry
	for _, file := range specified {
		stat, err := os.Stat(file)
		if err != nil {
			return err
		}
		entries = append(entries, entry{
			path: file,
			info: stat,
		})
	}
	counter := &errorCount{}
	for _, e := range entries {
		var err error
		switch e.info.IsDir() {
		case true:
			err = walk(e.path, opts, processor, counter)
		default:
			err = processFile(e.path, opts, processor, e.info, true, counter)
		}
		if err != nil {
			return err
		}
	}
	return counter.Error()
}

func processFile(path string, opts Options, p Processor, info fs.FileInfo, userSpecified bool, counter *errorCount) error {
	shouldProcess, err := opts.shouldProcess(path, info)
	if err != nil {
		return err
	}
	if !shouldProcess {
		return nil
	}
	shouldProcess = p.Matches(path, info, userSpecified)
	if !shouldProcess {
		return nil
	}
	err = counter.reportError(p.Process(path, info))
	if err != nil {
		if !opts.ContinueOnError {
			return errors.Wrap(err, path)
		}
	}
	return nil
}

func walk(root string, opts Options, p Processor, counter *errorCount) error {
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return processFile(path, opts, p, info, false, counter)
	})
}
