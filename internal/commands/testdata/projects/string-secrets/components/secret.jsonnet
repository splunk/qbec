{
  apiVersion: 'v1',
  kind: 'Secret',
  metadata: {
    name: 'my-secret',
  },
  stringData: {
    foo: std.extVar('secretValue'),
  },
}
