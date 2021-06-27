local foo = import 'foo.libsonnet';
local today = import 'data://today';

{
    apiVersion: "v1",
    kind: "ConfigMap",
    data: {
        foo: std.toString(foo),
    }
}
