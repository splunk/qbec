local base = import './base.libsonnet';

base {
    components +: {
        service2 +: {
            cpu: '50m',
        },
    }
}
