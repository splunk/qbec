/*
   Copyright 2019 Splunk Inc.

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

package commands

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
)

type initCommandConfig struct {
	cmd.AppContext
	withExample bool // create a hello world example
}

var baseParamsTemplate = template.Must(template.New("base").Parse(`
// this file has the baseline default parameters
{
  components: { {{- if .AddExample}}
    hello: {
      indexData: 'hello baseline\n',
      replicas: 1,
    }, {{- end}}
  },
}
`))

var envParamsTemplate = template.Must(template.New("env").Parse(`
// this file has the param overrides for the default environment
local base = import './base.libsonnet';

base {
  components +: { {{- if .AddExample}}
    hello +: {
      indexData: 'hello default\n',
      replicas: 2,
    }, {{- end}}
  }
}
`))

var paramsTemplate = template.Must(template.New("any-env").Parse(`
// this file returns the params for the current qbec environment
// you need to add an entry here every time you add a new environment.

local env = std.extVar('qbec.io/env');
local paramsMap = {
  _: import './environments/base.libsonnet',
  default: import './environments/default.libsonnet',
};

if std.objectHas(paramsMap, env) then paramsMap[env] else error 'environment ' + env + ' not defined in ' + std.thisFile

`))

var componentExampleTemplate = template.Must(template.New("comp").Parse(`
local p = import '../params.libsonnet';
local params = p.components.hello;

[
  {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: {
      name: 'demo-config',
    },
    data: {
      'index.html': params.indexData,
    },
  },
  {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {
      name: 'demo-deploy',
      labels: {
        app: 'demo-deploy',
      },
    },
    spec: {
      replicas: params.replicas,
      selector: {
        matchLabels: {
          app: 'demo-deploy',
        },
      },
      template: {
        metadata: {
          labels: {
            app: 'demo-deploy',
          },
        },
        spec: {
          containers: [
            {
              name: 'main',
              image: 'nginx:stable',
              imagePullPolicy: 'Always',
              volumeMounts: [
                {
                  name: 'web',
                  mountPath: '/usr/share/nginx/html',
                },
              ],
            },
          ],
          volumes: [
            {
              name: 'web',
              configMap: {
                name: 'demo-config',
              },
            },
          ],
        },
      },
    },
  },
]
`))

func writeTemplateFile(file string, t *template.Template, data interface{}) error {
	var w bytes.Buffer
	if err := t.Execute(&w, data); err != nil {
		return fmt.Errorf("unable to expand template for file %s, %v", file, err)
	}
	if err := ioutil.WriteFile(file, w.Bytes(), 0644); err != nil {
		return err
	}
	sio.Noticeln("wrote", file)
	return nil
}

func writeFiles(dir string, app model.QbecApp, config initCommandConfig) error {
	if err := os.Mkdir(app.Metadata.Name, 0755); err != nil {
		return err
	}

	templateData := struct {
		AddExample bool
	}{config.withExample}

	type templateFile struct {
		t *template.Template
		f string
	}

	compsDir, envDir := filepath.Join(dir, "components"), filepath.Join(dir, "environments")
	for _, dir := range []string{compsDir, envDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	files := []templateFile{
		{
			t: paramsTemplate,
			f: filepath.Join(dir, "params.libsonnet"),
		},
		{
			t: baseParamsTemplate,
			f: filepath.Join(envDir, "base.libsonnet"),
		},
		{
			t: envParamsTemplate,
			f: filepath.Join(envDir, "default.libsonnet"),
		},
	}
	if config.withExample {
		files = append(files, templateFile{
			t: componentExampleTemplate,
			f: filepath.Join(compsDir, "hello.jsonnet"),
		})
	}
	for _, tf := range files {
		if err := writeTemplateFile(tf.f, tf.t, templateData); err != nil {
			return err
		}
	}

	b, err := yaml.Marshal(app)
	if err != nil {
		return fmt.Errorf("yaml marshal: %v", err)
	}
	file := filepath.Join(dir, "qbec.yaml")
	if err := ioutil.WriteFile(file, b, 0644); err != nil {
		return err
	}
	sio.Noticeln("wrote", file)
	return nil
}

func doInit(args []string, config initCommandConfig) error {
	if len(args) != 1 {
		return fmt.Errorf("a single app name argument must be supplied")
	}
	name := args[0]
	_, err := os.Stat(name)
	if err == nil {
		return fmt.Errorf("directory %s already exists", name)
	}
	if !os.IsNotExist(err) {
		return err
	}

	ctx, err := config.KubeContextInfo()
	if err != nil {
		sio.Warnf("could not get current K8s context info, %v\n", err)
		sio.Warnln("using fake parameters for the default environment")
		ctx = &remote.ContextInfo{
			ServerURL: "https://minikube",
		}
	}
	sio.Noticef("using server URL %q and default namespace %q for the default environment\n", ctx.ServerURL, ctx.Namespace)
	app := model.QbecApp{
		Kind:       "App",
		APIVersion: model.LatestAPIVersion,
		Metadata: model.AppMeta{
			Name: name,
		},
		Spec: model.AppSpec{
			Environments: map[string]model.Environment{
				"default": {
					Server:           ctx.ServerURL,
					DefaultNamespace: ctx.Namespace,
				},
			},
		},
	}
	return writeFiles(name, app, config)
}

func newInitCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:   "init <app-name>",
		Short: "initialize a qbec app",
	}

	config := initCommandConfig{}
	c.Flags().BoolVar(&config.withExample, "with-example", false, "create a hello world sample component")

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doInit(args, config))
	}
	return c
}
