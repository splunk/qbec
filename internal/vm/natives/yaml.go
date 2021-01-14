package natives

import (
	"io"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// ParseYAMLDocuments parses the contents of the reader into an array of
// objects, one for each non-null document in the input.
func ParseYAMLDocuments(reader io.Reader) ([]interface{}, error) {
	ret := []interface{}{}
	d := yaml.NewYAMLToJSONDecoder(reader)
	for {
		var doc interface{}
		if err := d.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if doc != nil {
			ret = append(ret, doc)
		}
	}
	return ret, nil
}
