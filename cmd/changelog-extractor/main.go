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
