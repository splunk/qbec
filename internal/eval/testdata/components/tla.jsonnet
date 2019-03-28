function (suffix='v1') {
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: {
    name: 'tla-' + suffix,
  },
  data: {
    foo: 'bar',
  }
}

