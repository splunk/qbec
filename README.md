qbec
---

[![Build Status](https://circleci.com/gh/splunk/qbec.svg?style=svg)](https://circleci.com/gh/splunk/qbec)
[![Go Report Card](https://goreportcard.com/badge/github.com/splunk/qbec)](https://goreportcard.com/report/github.com/splunk/qbec)


Qbec (pronounced like the [Canadian province](https://en.wikipedia.org/wiki/Quebec)) is a CLI tool that 
allows you to create Kubernetes objects on multiple Kubernetes clusters or namespaces configured correctly for 
the target environment in question.

It is based on [jsonnet](https://jsonnet.org) and is similar to other tools in the same space like 
[kubecfg](https://github.com/ksonnet/kubecfg) and [ksonnet](https://ksonnet.io/). 

For more info, [read the docs](https://qbec.io/)

### Building from source

```shell
mkdir -p ${GOPATH}/src/github.com/splunk
cd ${GOPATH}/src/github.com/splunk && git clone git@github.com:splunk/qbec
cd qbec
make install  # installs dep, golint etc.
make
```







    

