{
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: {
        name: 'cm2',
    },
    data: {
        foo: std.extVar('extFoo'),
        bar: importstr 'data://myds',
    },
}
