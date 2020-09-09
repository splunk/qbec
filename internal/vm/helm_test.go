package vm

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelmOptions(t *testing.T) {
	a := assert.New(t)
	var h helmOptions
	a.Nil(h.toArgs())
	h = helmOptions{
		Execute:     []string{"a.yaml", "b.yaml"},
		KubeVersion: "1.10",
		Name:        "foo",
		Namespace:   "foobar",
		ThisFile:    "/path/to/my.jsonnet",
		Verbose:     true,
	}
	a.EqualValues([]string{
		"--execute", "a.yaml",
		"--execute", "b.yaml",
		"--kube-version", "1.10",
		"--namespace", "foobar",
	}, h.toArgs())
}

type cmOrSecret struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	Data map[string]string `json:"data"`
}

func TestHelmSimpleExpand(t *testing.T) {
	a := assert.New(t)
	jvm := New(Config{})
	file := "./consumer.jsonnet"
	inputCode := `
local expandHelmTemplate = std.native('expandHelmTemplate');

expandHelmTemplate(
    './testdata/charts/foobar',
    {
        foo: 'barbar',
    },
    {
        namespace: 'my-ns',
        nameTemplate: 'my-name',
        thisFile: std.thisFile,
		verbose: true,
    }
)
`
	code, err := jvm.EvaluateSnippet(file, inputCode)
	require.Nil(t, err)

	var output []cmOrSecret
	err = json.Unmarshal([]byte(code), &output)
	require.Nil(t, err)

	require.Equal(t, 2, len(output))

	sort.Slice(output, func(i, j int) bool {
		return output[i].Kind < output[j].Kind
	})

	ob := output[0]
	a.Equal("ConfigMap", ob.Kind)
	a.Equal("my-ns", ob.Metadata.Namespace)
	a.Equal("my-name", ob.Metadata.Name)
	a.Equal("barbar", ob.Data["foo"])
	a.Equal("baz", ob.Data["bar"])

	ob = output[1]
	a.Equal("Secret", ob.Kind)
	a.Equal("my-ns", ob.Metadata.Namespace)
	a.Equal("my-name", ob.Metadata.Name)
	a.Equal("Y2hhbmdlbWUK", ob.Data["secret"])
}

func TestHelmBadRelative(t *testing.T) {
	a := assert.New(t)
	jvm := New(Config{})
	file := "./consumer.jsonnet"
	inputCode := `
local expandHelmTemplate = std.native('expandHelmTemplate');

expandHelmTemplate(
    './testdata/charts/foobar',
    {
        foo: 'barbar',
    },
    {
        namespace: 'my-ns',
        name: 'my-name',
		verbose: true,
    }
)
`
	_, err := jvm.EvaluateSnippet(file, inputCode)
	require.NotNil(t, err)
	a.Contains(err.Error(), "exit status 1")
}
