external-data-app
---

Possibly the stupidest way to create a config map.

The point of this example is to show how to run an external program as part of qbec component evaluation.

A real example would run something more meaningful like `helm`, `istioctl`, or a script that talks to vault,
but that would need to pull in too many dependencies for our little demo.

The things to look for are:

* the computed variable called `cmdConfig` that shows the JSON that can be used to configure how an external
  program is invoked with arguments, environment variables and std input.
* the data source definition in [qbec.yaml](qbec.yaml) that defines a data source called `config-map` and associates the
  command configuration to it.
* the script [config-map.sh](config-map.sh) that uses its inputs to dump a config map object in YAML form
* the import in [components/my-config-map.jsonnet](components/my-config-map.jsonnet) that imports the data as a 
  string and returns the parsed YAML.
  
Run `qbec show local` to see the output.
