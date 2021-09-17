local expandHelmTemplate = std.native('expandHelmTemplate');

expandHelmTemplate(
  './charts/foobar',
  {
    foo: 'barbar',
  },
  {
    namespace: 'my-ns',
    nameTemplate: 'my-name',
    verbose: true,
  }
)
