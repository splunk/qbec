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

package natives

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
