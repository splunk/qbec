package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"
)

func main() {
	var input map[string]interface{}
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(b, &input)
	if err != nil {
		log.Fatalln(err)
	}
	data := map[string]interface{}{
		"ENV": map[string]interface{}{
			"DATA_SOURCE_NAME": os.Getenv("DATA_SOURCE_NAME"),
			"DATA_SOURCE_PATH": os.Getenv("DATA_SOURCE_PATH"),
		},
		"STDIN": input,
	}
	b, err = json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalln(err)
	}
	if os.Getenv("DATA_SOURCE_PATH") == "/fail" {
		log.Fatalln("failed data source lookup of path", os.Getenv("DATA_SOURCE_PATH"))
	}
	if os.Getenv("DATA_SOURCE_PATH") == "/slow" {
		time.Sleep(5 * time.Second)
	}
	fmt.Printf("%s\n", b)
}
