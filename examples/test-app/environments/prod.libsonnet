local base = import './base.libsonnet';

base {
    components +: {
        service1 +: {
            cpu: '1',
        },
        service2 +: {
            memory: '16Gi',
        }
    }
}
