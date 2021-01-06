local secret = {
    apiVersion: 'v1',
    kind: 'Secret',
    metadata: {
        name: 's1',
        annotations: {
            'directives.qbec.io/delete-policy': 'never',
        },
    },
    stringData: {
        foo: 'bar',
    },
};
if std.extVar('hide') then null else secret



