[
  {
    apiVersion: 'v1',
    kind: 'Namespace',
    metadata: {
      name: 'first',
    },
  },
  {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: {
      name: 'first-cm',
      namespace: 'first',
    },
    data: {
      foo: 'bar',
    },
  },
]
