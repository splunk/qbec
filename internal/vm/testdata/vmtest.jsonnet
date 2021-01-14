local lib = import 'foobar.libsonnet';
lib.makeFooBar(std.extVar('foo'), std.extVar('bar'))
