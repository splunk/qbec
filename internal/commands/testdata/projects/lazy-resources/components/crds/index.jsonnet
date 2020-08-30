local parseYaml = std.native('parseYaml');
local externals = import 'externals.libsonnet';
local suffix = externals.suffix;
local params = { foo: 'foo' + suffix, Foo: 'Foo' + suffix };
local crd = parseYaml(importstr './crd.yaml')[0];

crd + {
    metadata +: {
        name: '%(foo)ss.test.qbec.io' % params,
    },
    spec +: {
        names +: {
            kind: '%(Foo)s' % params,
            listKind: '%(Foo)sList' % params,
            plural: '%(foo)ss' % params,
            singular: '%(foo)s' % params,
        }
    }
}

