package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"time"
)

func main() {
	b, err := ioutil.ReadAll(os.Stdin)
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
