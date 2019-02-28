local objects = import 'objects.libsonnet';

{
    configMap: objects.configmap('bar-system','svc2-cm', { foo : 'bar' }),
    secret: objects.secret('bar-system','svc2-secret', { foo : std.base64('bar') }),
}

