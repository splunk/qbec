{
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: {
    name: std.extVar('computed.qbec.io/std-map2').name + '.' + std.extVar('computed.qbec.io/std-map2').name2
  },
  data: {
    foo: std.extVar('qbec.io/env'),
  }
}
