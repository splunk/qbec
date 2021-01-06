local base = import '_.libsonnet';

base {
    components +: {
        service2 +: {
            cpu: '50m',
        },
    }
}
