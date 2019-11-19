/*
   Copyright 2019 Splunk Inc.

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
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	log.SetOutput(os.Stderr)
	if len(os.Args) != 2 {
		log.Fatalln("Must pass the Changelog file name as the first argument")
	}
	filename := os.Args[1]
	if err := printReleaseNotes(filename); err != nil {
		log.Fatalln(err)
	}

}

func printReleaseNotes(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(f)
	var startPrint bool
	tagLinePrefix := "## v"
	for scanner.Scan() {
		line := scanner.Text()
		if startPrint && !strings.HasPrefix(line, tagLinePrefix) {
			fmt.Println(scanner.Text())
		}
		if strings.HasPrefix(line, tagLinePrefix) {
			if startPrint {
				// Exit at the next matching tag
				break
			}
			startPrint = true
		}
	}
	return scanner.Err()
}
