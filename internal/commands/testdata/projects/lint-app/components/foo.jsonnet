local foo = import 'foo.libsonnet';
local today = importstr 'data://today';
local s = import 'data://jsonstr';
local o = import 'data://object';
local a = import 'data://array';

{
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
        name: 'lint-test'
    },
    data: {
        foo: std.toString(foo),
        today: today,
        str: s,
        bar: o.bar,
        a: std.map(function (item) 'x:' + item, a),
    }
}
