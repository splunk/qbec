---
title: Tips and tricks
weight: 40
---

## Runtime

* qbec is written to have good performance even when dealing with hundreds of objects. That said,
  this is wholly dependent on how long a basic command like `qbec show` takes to execute. Most of the
  time taken by `qbec show` is in component evaluation, which in turn is dependent on the performance
  of jsonnet libraries that your components use. A good rule of thumb is that you will have an 
  enjoyable experience with qbec if `qbec show` executes in less than a second or two and a poorer
  experience otherwise.
  
* Organizing runtime parameters in the recommended manner will let you use the `param` subcommands
  effectively. In addition, restricting parameter values to simple scalar values, short arrays
  of scalar values or small, shallow objects will provide better listing and diffs.
  
## Development

* If you typically work with just one qbec app, set the `QBEC_ROOT` environment variable to the app
  directory so that qbec works from any working directory.

* Use the component and kind filters to restrict the scope of objects that are acted upon so that you
  see just the information you care about at any time.

* Set up your development environment (e.g. the VSCode IDE) with a jsonnet executable that can 
  parse YAML (qbec ships with the `jsonnet-qbec` command that provides all the native extensions it
  installs). Configure the `qbec.io/env` extension variable to a valid environment. With this in place,
  the IDE will be able to show you errors early during development, without even having to run any
  of the qbec commands. 

* Use the `--clean` option of the `show` command so you can see object contents without the additional qbec metadata.

* Declare a post-processor to add common metadata to all objects.
 
## Continuous Integration
 
 * Set the `QBEC_YES` environment variable to `true` so that all qbec prompts are disabled.
 
 * Use the `--wait` option of the `apply` command so that qbec waits for deployments to fully roll out. Your subsequent
   functional tests can then rely on the rollout to be complete before they start executing. This ensures that your
   pods under test are ready and are of the desired version.
   
 
