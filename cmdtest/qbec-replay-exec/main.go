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

// cmd qbec-replay-exec implements an exec provider that replays what was given to it in JSON format
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"time"
)

func main() {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalln(err)
	}
	exe := os.Args[0]
	args := os.Args[1:]
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("get wd: %v", err)
	}
	var env []string
	for _, v := range os.Environ() {
		env = append(env, v)
	}
	sort.Strings(env)
	data := map[string]interface{}{
		"dsName":  os.Getenv("__DS_NAME__"),
		"command": exe,
		"args":    args,
		"dir":     wd,
		"env":     env,
		"stdin":   string(b),
	}
	b, err = json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalln(err)
	}
	if os.Getenv("__DS_PATH__") == "/fail" {
		log.Fatalln("failed data source lookup of path", os.Getenv("__DS_PATH__"))
	}
	if os.Getenv("__DS_PATH__") == "/slow" {
		time.Sleep(5 * time.Second)
	}
	fmt.Printf("%s\n", b)
}
