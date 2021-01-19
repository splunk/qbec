package main

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVMWithDataSources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	dsURL := "exec://cdb?exe=./testdata/contacts-db.sh"
	args := []string{
		"jsonnet-qbec-test",
		"-V", "a=b",
		"--ext-code", "c=true",
		"--data-source", dsURL,
		"./testdata/people.jsonnet",
	}
	var out bytes.Buffer
	err := run(args, &out)
	require.NoError(t, err)
	t.Log(out.String())
}
