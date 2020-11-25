local foo = std.extVar('foo');
local cm = {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: {
        name: 'cm1',
        annotations: {
            'directives.qbec.io/update-policy': 'never',
        },
    },
    data: {
        foo: foo,
    },
};
cm

