package vm

import (
	"encoding/json"
	"io"
)

// ParseJSON parses the contents of the reader into an data object and returns it.
func ParseJSON(reader io.Reader) (interface{}, error) {
	dec := json.NewDecoder(reader)
	var data interface{}
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}
