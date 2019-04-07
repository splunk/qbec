function (tlaFoo = 'bar') (
    {
        apiVersion: 'v1',
        kind: 'ConfigMap'
        metadata: {
            name: 'cm1',
        },
        data: {
            foo: tlaFoo,
        },
    }
)

