function (foo, bar) {
  apiVersion: "v1",
  kind: "ConfigMap",
  metadata: {
    name: "tla-config-map"
  },
  data: {
    foo: foo,
    bar: if bar then 'yes' else 'no',
  }
}
