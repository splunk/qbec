local expandHelmTemplate = std.native('expandHelmTemplate');

expandHelmTemplate(
    './charts/foobar',
    {
        foo: 'barbar',
    },
    {
        namespace: 'my-ns',
        name: 'my-name',
		verbose: true,
    }
)
