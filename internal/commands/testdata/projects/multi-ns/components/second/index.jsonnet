[
  {
    apiVersion: 'v1',
    kind: 'Namespace',
    metadata: {
      name: 'second',
    },
  },
  {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: {
      name: 'second-cm',  // don't specify a namespace, use default
    },
    data: {
      foo: 'bar',
    },
  },
  {
    apiVersion: 'v1',
    kind: 'Secret',
    metadata: {
      name: 'second-secret',
      namespace: 'second',
    },
    stringData: {
      foo: 'bar',
    },
  },
]
