local wait = std.extVar('wait');
local annotations = if !wait then { 'directives.qbec.io/wait-policy': 'never' } else {};

{
  apiVersion: 'apps/v1',
  kind: 'Deployment',
  metadata: {
    annotations: annotations,
    labels: {
      app: 'nginx',
    },
    name: 'nginx-by-wait',
  },
  spec: {
    replicas: 1,
    selector: {
      matchLabels: {
        app: 'nginx',
      },
    },
    strategy: {
      type: 'RollingUpdate',
    },
    template: {
      metadata: {
        labels: {
          app: 'nginx',
        },
      },
      spec: {
        containers: [
          {
            image: 'nginx',
            imagePullPolicy: 'Always',
            name: 'nginx',
          },
        ],
      },
    },
  },
}
