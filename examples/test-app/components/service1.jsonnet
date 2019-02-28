local objects = import 'objects.libsonnet';

{
    configMap: objects.configmap('foo-system','svc1-cm', { foo : 'bar' }),
}
