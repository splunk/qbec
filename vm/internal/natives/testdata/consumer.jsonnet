local expandHelmTemplate = std.native('expandHelmTemplate');

expandHelmTemplate(
  './charts/foobar',
  {
    foo: 'barbar',
  },
  {
    namespace: 'my-ns',
    nameTemplate: 'my-name',
    thisFile: std.thisFile,
    verbose: true,
    apiVersions: [
      'networking.k8s.io/v1/Ingress',
    ],
  }
)
