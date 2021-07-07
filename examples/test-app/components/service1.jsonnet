local objects = import 'objects.libsonnet';
local fooValue = std.extVar('externalFoo');

{
  configMap: objects.configmap('foo-system', 'svc1-cm', { foo: fooValue }),
}
