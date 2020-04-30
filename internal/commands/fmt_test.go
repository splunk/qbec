package commands

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsYaml(t *testing.T) {
	var tests = []struct {
		fileName string
		expected bool
	}{
		{"testdata/qbec.yaml", true},
		{"testdata/test.yml", true},
		{"testdata", false},
		{"testdata/components/c1.jsonnet", false},
		{"testdata/test.libsonnet", false},
	}
	for _, test := range tests {
		t.Run(test.fileName, func(t *testing.T) {
			f, err := os.Stat(test.fileName)
			if err != nil {
				t.Fatalf("Unexpected error'%v'", err)
			}
			var actual = isYamlFile(f)
			if test.expected != actual {
				t.Errorf("Expected '%t', got '%t'", test.expected, actual)
			}
		})
	}
}

func TestShouldFormat(t *testing.T) {
	var tests = []struct {
		fileName string
		formats  []string
		expected bool
	}{
		{"testdata/qbec.yaml", []string{"yaml"}, true},
		{"testdata/test.yml", []string{"jsonnet"}, false},
		{"testdata", []string{"jsonnet", "yaml"}, false},
		{"testdata/components/c1.jsonnet", []string{"jsonnet"}, true},
	}
	for _, test := range tests {
		t.Run(test.fileName, func(t *testing.T) {
			f, err := os.Stat(test.fileName)
			if err != nil {
				t.Fatalf("Unexpected error'%v'", err)
			}
			for _, format := range test.formats {
				var actual = shouldFormat(format, f)
				if test.expected != actual {
					t.Errorf("Expected '%t', got '%t'", test.expected, actual)
				}
			}

		})
	}
}
func TestIsJsonnet(t *testing.T) {
	var tests = []struct {
		fileName string
		expected bool
	}{
		{"testdata/components/c1.jsonnet", true},
		{"testdata/test.libsonnet", true},
		{"testdata", false},
		{"testdata/qbec.yaml", false},
		{"testdata/test.yml", false},
	}
	for _, test := range tests {
		t.Run(test.fileName, func(t *testing.T) {
			f, err := os.Stat(test.fileName)
			if err != nil {
				t.Fatalf("Unexpected error'%v'", err)
			}
			var actual = isJsonnetFile(f)
			if test.expected != actual {
				t.Errorf("Expected '%t', got '%t'", test.expected, actual)
			}
		})
	}
}

func TestDoFmt(t *testing.T) {
	var b bytes.Buffer
	var tests = []struct {
		args        []string
		config      fmtCommandConfig
		expectedErr string
	}{
		{[]string{"a", "b"}, fmtCommandConfig{}, `unexpected format arguments: ["a" "b"]`},
		{[]string{"a"}, fmtCommandConfig{}, `invalid format file format: "a"`},
		{[]string{}, fmtCommandConfig{check: true, write: true}, `check and write are not supported together`},
		{[]string{}, fmtCommandConfig{files: []string{"nonexistentfile"}}, `stat nonexistentfile: no such file or directory`},
		{[]string{}, fmtCommandConfig{Config: &Config{stdout: &b}, files: []string{"testdata/qbec.yaml"}}, ""},
		{[]string{}, fmtCommandConfig{Config: &Config{stdout: &b}, files: []string{"testdata/components"}}, ""},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test%d", i), func(t *testing.T) {
			var err = doFmt(test.args, &test.config)
			if test.expectedErr == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				if test.expectedErr != err.Error() {
					t.Errorf("Expected %v but got %v", test.expectedErr, err.Error())
				}
			}
		})
	}

}

func TestFormatYaml(t *testing.T) {
	var testfile, err = ioutil.ReadFile("testdata/test.yml")
	require.Nil(t, err)
	o, err := formatYaml(testfile)
	require.Nil(t, err)
	e, err := ioutil.ReadFile("testdata/test.yml.formatted")
	require.Nil(t, err)
	if !bytes.Equal(o, e) {
		t.Errorf("Expected %q, got %q", string(e), string(o))
	}

	var tests = []struct {
		input     []byte
		expectErr bool
	}{
		{input: nil, expectErr: false},
		{input: []byte("abc"), expectErr: false},
		{input: []byte("---"), expectErr: false},
		{input: []byte("---\nnull\n---"), expectErr: false},
		{input: []byte("*abc*"), expectErr: true},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("test%d", i), func(t *testing.T) {
			var _, err = formatYaml(test.input)
			if test.expectErr {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestFormatJsonnet(t *testing.T) {
	var testfile, err = ioutil.ReadFile("testdata/test.libsonnet")
	require.Nil(t, err)
	o, err := formatJsonnet(testfile)
	require.Nil(t, err)
	e, err := ioutil.ReadFile("testdata/test.libsonnet.formatted")
	require.Nil(t, err)
	if !bytes.Equal(o, e) {
		t.Errorf("Expected %q, got %q", string(e), string(o))
	}
}

func TestFmtCommand(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("alpha", "fmt", "-f", "prod-env.yaml")
	require.Nil(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`      - service2`))
}

func TestProcessFile(t *testing.T) {

	var tests = []struct {
		input  string
		output string
	}{
		{input: "testdata/test.libsonnet", output: "testdata/test.libsonnet.formatted"},
		{input: "testdata/test.yml", output: "testdata/test.yml.formatted"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			var b bytes.Buffer
			var config = fmtCommandConfig{}
			var err = processFile(&config, test.input, nil, &b)
			require.Nil(t, err)
			e, err := ioutil.ReadFile(test.output)
			require.Nil(t, err)
			var o = b.Bytes()
			if !bytes.Equal(e, o) {
				t.Errorf("Expected %q, got %q", string(e), string(o))
			}
		})
	}
}

// Adapted from https://golang.org/src/cmd/gofmt/gofmt_test.go
func TestBackupFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "qbecfmt_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	name, err := backupFile(filepath.Join(dir, "foo.yaml"), []byte("a: 1"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Created: %s", name)
}
