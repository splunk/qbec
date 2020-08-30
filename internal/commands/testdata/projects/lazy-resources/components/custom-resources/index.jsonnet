local parseYaml = std.native('parseYaml');
local externals = import 'externals.libsonnet';
local suffix = externals.suffix;
local params = { foo: 'foo' + suffix, Foo: 'Foo' + suffix };
local resources = parseYaml(importstr './res.yaml');

std.map(function (obj) obj + { kind: '%(Foo)s' % params }, resources)
