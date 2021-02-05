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

package cmd

import (
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/sio"
)

type profiler struct {
	cpuProfile    string
	memoryProfile string
	cpu           *os.File
	memory        *os.File
}

func (p *profiler) init() error {
	var err error
	if p.cpuProfile != "" {
		p.cpu, err = os.Create(p.cpuProfile)
		if err != nil {
			return err
		}
		err = pprof.StartCPUProfile(p.cpu)
		if err != nil {
			return errors.Wrap(err, "start CPU profile")
		}
		sio.Debugln("profiling CPU to file", p.cpuProfile)
	}
	if p.memoryProfile != "" {
		p.memory, err = os.Create(p.memoryProfile)
		if err != nil {
			return err
		}
		sio.Debugln("profiling memory to file", p.memoryProfile)
	}
	return nil
}

func (p *profiler) Close() error {
	var cpuError, writeError, memError error
	if p.cpu != nil {
		sio.Debugln("stop CPU profile")
		pprof.StopCPUProfile()
		cpuError = p.cpu.Close()
	}
	if p.memory != nil {
		runtime.GC()
		err := pprof.WriteHeapProfile(p.memory)
		sio.Debugln("write memory profile")
		if err != nil {
			writeError = errors.Wrap(err, "write memory profile")
		}
		memError = p.memory.Close()
	}
	if cpuError != nil {
		return cpuError
	}
	if writeError != nil {
		return writeError
	}
	return memError
}

func addProfilerOptions(cmd *cobra.Command, prefix string) func() (*profiler, error) {
	var p profiler
	pf := cmd.PersistentFlags()
	pf.StringVar(&p.cpuProfile, prefix+"cpu", "", "filename to write CPU profile")
	pf.StringVar(&p.memoryProfile, prefix+"memory", "", "filename to write memory profile")
	return func() (*profiler, error) {
		if err := p.init(); err != nil {
			_ = p.Close()
			return nil, errors.Wrap(err, "init profiler")
		}
		return &p, nil
	}
}
