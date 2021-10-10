package helm3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateOptionsEmpty(t *testing.T) {
	tc := TemplateOptions{}
	parts := tc.toCommandLine()
	require.Equal(t, 0, len(parts))
}

func TestTemplateOptionsFull(t *testing.T) {
	tc := TemplateOptions{
		Namespace:                "default",
		CreateNamespace:          true,
		APIVersions:              []string{"1.2", "2.1"},
		DependencyUpdate:         true,
		Description:              "foo bar",
		Devel:                    true,
		DisableOpenAPIValidation: true,
		GenerateName:             true,
		IncludeCRDs:              true,
		IsUpgrade:                true,
		SkipCRDs:                 true,
		SkipTests:                true,
		InsecureSkipTLSVerify:    true,
		KubeVersion:              "1.16",
		NameTemplate:             "gentemp",
		NoHooks:                  true,
		PassCredentials:          true,
		Password:                 "foobar",
		RenderSubchartNotes:      true,
		Replace:                  true,
		Repo:                     "r1",
		ShowOnly:                 []string{"foo", "bar"},
		Username:                 "me",
		Validate:                 true,
		Verify:                   true,
		Version:                  "1.2.3",
	}
	ret := tc.toCommandLine()
	a := assert.New(t)
	a.Contains(ret, "--namespace=default")
	a.Contains(ret, "--api-versions=1.2")
	a.Contains(ret, "--api-versions=2.1")
	a.Contains(ret, "--kube-version=1.16")
	a.Contains(ret, "--name-template=gentemp")
	a.Contains(ret, "--password=foobar")
	a.Contains(ret, "--repo=r1")
	a.Contains(ret, "--show-only=foo")
	a.Contains(ret, "--show-only=bar")
	a.Contains(ret, "--username=me")
	a.Contains(ret, "--version=1.2.3")
	for _, s := range []string{
		"create-namespace",
		"dependency-update",
		"devel",
		"disable-openapi-validation",
		"generate-name",
		"include-crds",
		"is-upgrade",
		"skip-crds",
		"skip-tests",
		"insecure-skip-tls-verify",
		"no-hooks",
		"pass-credentials",
		"render-subchart-notes",
		"replace",
		"validate",
		"verify",
	} {
		a.Contains(ret, "--"+s)
	}

	str := tc.toDisplay()
	a.Contains(str, "--namespace=default")
	a.Contains(str, "--api-versions=1.2")
	a.Contains(str, "--api-versions=2.1")
	a.Contains(str, "--kube-version=1.16")
	a.Contains(str, "--name-template=gentemp")
	a.Contains(str, "--password=<REDACTED>")
	a.Contains(str, "--repo=r1")
	a.Contains(str, "--show-only=foo")
	a.Contains(str, "--show-only=bar")
	a.Contains(str, "--username=me")
	a.Contains(str, "--version=1.2.3")
	for _, s := range []string{
		"create-namespace",
		"dependency-update",
		"devel",
		"disable-openapi-validation",
		"generate-name",
		"include-crds",
		"is-upgrade",
		"skip-crds",
		"skip-tests",
		"insecure-skip-tls-verify",
		"no-hooks",
		"pass-credentials",
		"render-subchart-notes",
		"replace",
		"validate",
		"verify",
	} {
		a.Contains(str, "--"+s)
	}
}
