local objects = import 'objects.libsonnet';

function (tlaFoo = 'bar') (
    {
        configMap: objects.configmap('bar-system','svc2-cm', { foo : tlaFoo }),
        secret: objects.secret('bar-system','svc2-secret', { foo : std.base64('bar') }),
    }
)

