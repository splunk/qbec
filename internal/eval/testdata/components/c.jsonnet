{
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: {
    name: 'jsonnet-config-map'
  },
  data: {
    foo: std.extVar('qbec.io/env')
  }
}
