local base = import '_.libsonnet';

base {
  components+: {
    service1+: {
      cpu: '1',
    },
    service2+: {
      memory: '16Gi',
    },
  },
}
