{
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: {
    name: 'foobar',
  },
  data: {
    foo: std.extVar('qbec.io/env'),
  }
}
