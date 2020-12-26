{
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: {
    name: std.extVar('computed.qbec.io/std-map').name
  },
  data: {
    foo: std.extVar('qbec.io/env'),
  }
}
