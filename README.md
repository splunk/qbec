![qbec](site/static/images/qbec-logo-black.svg)

[![Github build status](https://github.com/splunk/qbec/workflows/build/badge.svg)](https://github.com/splunk/qbec/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/splunk/qbec)](https://goreportcard.com/report/github.com/splunk/qbec)
[![codecov](https://codecov.io/gh/splunk/qbec/branch/main/graph/badge.svg)](https://codecov.io/gh/splunk/qbec)
[![GolangCI](https://golangci.com/badges/github.com/splunk/qbec.svg)](https://golangci.com/r/github.com/splunk/qbec)


[![Build Stats](https://buildstats.info/github/chart/splunk/qbec?branch=main)](https://buildstats.info/github/chart/splunk/qbec?branch=main)


Qbec (pronounced like the [Canadian province](https://en.wikipedia.org/wiki/Quebec)) is a CLI tool that 
allows you to create Kubernetes objects on multiple Kubernetes clusters or namespaces configured correctly for 
the target environment in question.

It is based on [jsonnet](https://jsonnet.org) and is similar to other tools in the same space like 
[kubecfg](https://github.com/ksonnet/kubecfg) and [ksonnet](https://ksonnet.io/). 

For more info, [read the docs](https://qbec.io/)

### Installing

Use a prebuilt binary [from the releases page](https://github.com/splunk/qbec/releases) for your operating system.

On MacOS, you can install qbec using homebrew:

```
$ brew tap splunk/tap 
$ brew install qbec
```

### Building from source

```shell
git clone git@github.com:splunk/qbec
cd qbec
make install  # installs lint tools etc.
make
```
