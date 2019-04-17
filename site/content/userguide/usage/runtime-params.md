---
title: Runtime parameters
weight: 20
---

Runtime parameters are values that differ across environments, change over time, or are secrets that should not
be casually revealed.

Properties like replicas of a deployment per environment, cluster level endpoints etc. are usually known in advance and
should be checked into source code.

Some parameters cannot be checked into source code. These include tags for images produced by a CI build whose value
should be subsequently used in the same build for a deployment, environment-specific secrets, etc.

qbec provides the following facilities for runtime parameters.

* it exposes the external variable `qbec.io/env` that contains the name of the environment used for the command.
  This mechanism allows you to parameterize values that are known in advance for an environment.
* it allows you to declare jsonnet _external variables_ in `qbec.yaml` with default values. Values for external
  variables can be supplied on the command line. Default values are used when declared variables have not be set.
  This mechanism allows you to develop components locally without having to specify variables on the command line
  for every invocation and still set them explicitly when you actually apply the objects to the cluster.
* it allows you to declare jsonnet _top-level variables_ in `qbec.yaml` and associate them with components to which
  they should be passed. In this case, you set defaults for these variables in the jsonnet code itself.

The [jsonnet tutorial](https://jsonnet.org/learning/tutorial.html#parameterize-entire-config) explains external
and top-level variables in detail along with the pros and cons of each.

Let's examine each of these in turn.

## Using qbec.io/env

You can access this variable using the `std.extVar` function of the jsonnet standard library and customize parameters.
This allows you to create a parameters object that can be imported by your components.

```jsonnet
local env = std.extVar('qbec.io/env'); // get the value of the environment

// define baseline parameters
local baseParams = {
    components: {
        service1: {
            replicas: 1,
        },
    },
};

// define parameters per environment
local paramsByEnv = {

    _: baseParams, // the baseline environment used by qbec for some commands

    dev: baseParams {
        components+: {
            service1+: {
                replicas: 2,
            },
        },
    },

    prod: baseParams {
        components+: {
            service1+: {
                replicas: 3,
            },
        },
    },
};

// return value for correct environment
if std.objectHas(paramsByEnv, env) then paramsByEnv[env] else error 'environment ' + env ' not defined in ' + std.thisFile
 
``` 

When there are many components and values, the above code can be split up into multiple files with each file 
returning the object for a specific environment and the top-level file then `import`s the other files.

Since variables and values are checked into source code when using this method, this is the most explicit way to
set parameters and should be used as much as possible.

## Using jsonnet external variables

You declare the external variable(s) that you expect in `qbec.yaml`.

```yaml
spec:
    vars:
      external:
        - name: service1_image_tag
          default: latest

        - name: service1_secret
          default: changeme # fake value
          secret: true # qbec debug logs will not print this value in cleartext
```

then you can use them in your components or runtime parameter config object, like so:

```jsonnet
    local imageTag = std.extVar('service1_image_tag');
```

and specify real values on the qbec command-line:

```bash
export service_secret=XXX
qbec apply dev --vm:ext-str service1_image_tag=1.0.3 --vm:ext-str service1_secret
```

Note that we are explicitly setting the image tag to 1.0.3 but not providing the value for the secret on the command
line. This causes qbec to set the secret value from the environment variable with the same name. This is the preferred
method for passing in secrets.

## Using top-level jsonnet variables

You declare the top-level variable(s) used in your code associating them with the components that need them.


```yaml
spec:
    vars:
      topLevel:
        - name: service1Tag
          components: [ 'service1' ]

        - name: service1Secret
          components: [ 'service1' ]
          secret: true # qbec debug logs will not print this value in cleartext
```

In this case you would code your `service1.jsonnet` component as follows:

```jsonnet
function (serviceTag='latest', serviceSecret='changeme') (
    // for example: return an array containing a seret and a deployment
    [
        {
            apiVersion: 'v1',
            kind: 'Secret',
            metadata: {
                name: 'my-secret',
            },
            data: {
                token: std.base64(serviceSecret),
            },
        },
        // ... etc
    ]
)
```

and specify real values on the qbec command-line:

```bash
export service_secret=XXX
qbec apply dev --vm:tla-str service1_image_tag=1.0.3 --vm:tla-str service1_secret
```

### Notes on usage

* Values (and defaults) need not be strings. They can be numbers, booleans or objects. In this case, you need to use the 
  `--vm:ext-code foo=true` or `--vm:tla-code foo=true` syntax. In the preceding examples, this will pass in a 
  true value that is a boolean instead of the string "true". Use this judiciously. Requiring code variables might limit 
  your ability to define such variables in your IDE which may only allow string variables.
* Excessive use of variables introduces cognitive overhead and reliance on qbec's defaulting features.
  A few declarations are ok but think twice before defining tens of such variables.
* Prefer to declare all external variables used by your components (qbec doesn't _require_ you to define them) so that
  the external interface of your app is documented. This will also allow you to enable strict mode.
* You cannot share the name of a top-level variable across different components to mean different things. For example,
  you cannot have a top-level argument called `imageTag` for 2 different services that need different image tags.
* Defaults for external variables cannot be `null`. This is because qbec cannot tell the difference between a 
  default that was not specified versus one that was explicitly set to `null`.

## Strict mode

The `--strict-vars` flag for qbec commands can help you ensure correctness of your qbec command line invocation.
When this option is set, qbec:

* requires _all_ declared values to be set on the command line
* disallows undeclared variables to be set 

This is a good option to set for CI builds to ensure that the build breaks if someone introduces a new variable
but fails to set a value for it for the `apply` command, or if the variable name has a typo.



