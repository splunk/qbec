{
  objects: import 'data://helm/apache?config-from=apache-config',
  config: {
    name: 'mock-release',
    options: {
      repo: 'https://charts.bitnami.com/bitnami',
      version: '11.2.17',
      namespace: 'foobar',
    },
    values: {
      key: 'value',
    },
  },
}
