package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var input map[string]interface{}

func handler(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	data := map[string]interface{}{
		"ENV": map[string]interface{}{
			"DATA_SOURCE_NAME": os.Getenv("DATA_SOURCE_NAME"),
			"DATA_SOURCE_PATH": p,
		},
		"STDIN": input,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if p == "/fail" {
		http.Error(w, "no data for you", http.StatusBadRequest)
		return
	}
	if p == "/slow" {
		time.Sleep(2 * time.Second)
	}
	_, _ = w.Write(b)
}

func main() {
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(b, &input)
	if err != nil {
		log.Fatalln(err)
	}
	_ = http.ListenAndServe(fmt.Sprintf("127.0.0.1:%s", os.Getenv("DATA_SOURCE_PORT")), http.HandlerFunc(handler))
}
