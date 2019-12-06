{
    components: {
        base: {
            env: std.extVar('qbec.io/env'),
            ns: std.extVar('qbec.io/defaultNs'),
            tag: std.extVar('qbec.io/tag'),
            foo: std.extVar('qbec.io/envProperties').foo,
        }
    }
}
