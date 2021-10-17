---
title: Jsonnet data importer
weight: 195
---

Sometimes you need to generate data using external programs and be able to access it in your jsonnet components.
This can be accomplished using the data source importer that ships with qbec.

While the design of the importer allows for tight, native integration with tools like `helm`, `istioctl`, `kustomize`,
and secret engines like `vault`, the only integration that is currently implemented is `exec` that allows you to
run external programs and use the standard output they produce as data in jsonnet code.

The [sample data app](https://github.com/splunk/qbec/tree/main/examples/external-data-app) provides a working
implementation of such an importer and demonstrates everything that you need to do to set it up.

The recipe for pulling in additional data from external command output consists of 3 parts:

### Step 1: Specify a configuration for command invocation

This includes program name, arguments, environment variables, and standard input. This is a JSON object with the following properties:

```json
{
  "command": "/path/to/executable",
  "args": [ "array", "of", "positional", "arguments"],
  "env": {
    "map": "of",
    "environment": "variables",
  },
  "stdin": "standard input to program as a string",
  "inheritEnv": false,
  "timeout": "10s"
}
```
You construct this object using an external code variable that can be defined in qbec.yaml.
Now, some arguments can be different based on the qbec environment.
This can be accomplished using computed variables that qbec supports. Computed variables are defined as follows:

```yaml
spec:
    vars:
      computed:
        - name: cmdConfig
          code: |
            {
              command: 'myCommand.sh',
              args: [ '--env', std.extVar('qbec.io/env') ],
            }
```

In the example above, the arguments to the command are constructed dynamically based on the qbec environment.

### Step 2: Create a data source that refers to this configuration

Add a "data source" to qbec.yaml as follows:

```yaml
spec:
  dataSources:
    - exec://my-data-source?configVar=cmdConfig
```

The above URL has 3 parts.

* The scheme of the URL is `exec` which is the kind of data source that is being created. Currently, only `exec` is
  allowed, but we could add more schemes in the future for native integrations with specific tools.
* The hostname in the URL is a name of your choosing. This is the data source name.
* The `configVar` query parameter is the reference to the configuration variable using which the data source is
  initialized.

### Step 3: use this data source in your jsonnet component code

```jsonnet
import 'data://my-data-source/some/path'
```

* The `data` scheme in the URI above allows qbec to determine that you want to import external data.
* The hostname in the URI is the name of the data source that you declared.
* The path in the URI is passed to the command as an environment variable called `__DS_PATH__`.
* The command also gets another environment variable called `__DS_NAME__` that is set to the data source name.

A simple command may not respect the `__DS_PATH__` environment variable and always output the same data.
On the other hand, you can write a more complex integration (say, with Vault) by having the command use the
information in the `__DS_PATH__` variable and emit secrets specific to that path.

## Usage notes

* Commands should output valid JSON or jsonnet when using `import data://my-source` but they can output any string
  (e.g. YAML) when called as `importstr data://my-source`. It is up to you to post-process the output (e.g. by using
  `parseYaml`) in the latter case.

* Commands should behave like functions providing the same output for the same set of inputs. In particular,
  commands **should not write files** in the jsonnet source tree.

* Commands are run using Go's `os/exec` package which means that they do **not** run under a shell.
  You cannot use pipes and redirection.
  If you want this functionality you need to run the command as
  `[ 'sh', '-c', 'subcommand1 | subcommand2']`.
  Better yet, write a shell script that does all this.

* You are responsible for ensuring that the executable you are running comes from a trusted source. Do not pull
  down executables off the internet without checking their SHA sums.

* In like manner, you are responsible to ensure that everyone on the team is the using the same version of the
  command that is invoked. One strategy is to download the command you want into a `.bin` directory using a
  `Makefile` and run it from there. The Makefile downloads the same version of the command for different OS
  environments and checks their SHA sums.

* The command that is run does **not** inherit the OS environment from the qbec process unless `inheritEnv` is set to true.
  Only the environment variables explicitly defined in the config, as well as `__DS_NAME__` and `__DS_PATH__` are set.
