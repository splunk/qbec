package bulkfiles

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/sio"
)

// Processor indicates whether a file matches for processing, and allows some arbitrary processing on it
// It is only ever given files, never directories for processing.
type Processor interface {
	// Matches returns true if the specified path should be processed. The `userSpecified` argument
	// indicates that the file was explicitly passed in by the user.
	Matches(path string, file fs.FileInfo, userSpecified bool) bool
	// Process processes the specified file and returns an error in case of processing errors.
	// Note that this error will not be reported to the user directly. It is up to the processor
	// to indicate errors to the user by printing it.
	Process(path string, file fs.FileInfo) error
}

// Options are options for processing.
type Options struct {
	ContinueOnError bool
}

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
	if e.numErrors == 0 {
		return nil
	}
	return fmt.Errorf("%d errors encountered", e.numErrors)
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
		sio.Println(err.Error())
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
