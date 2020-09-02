{
    configmap(namespace, name, vars={}):: {
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: {
            namespace: namespace,
            name: name,
            labels: {
                name: name,
            },
        },
        data: vars,
    },
    secret(namespace,name,vars={}):: {
        apiVersion: "v1",
        kind: "Secret",
        metadata: {
            namespace: namespace,
            name: name,
            labels: {
                name: name,
            },
        },
        data: vars,
    },
    deployment(namespace, name, image):: {
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        metadata: {
            namespace: namespace,
            name: name,
            labels: {
                name: name,
            },
        },
        spec: {
            selector: {
                matchLabels: {
                    name: name,
                },
            },
            template: {
                metadata: {
                    labels: {
                        name: name,
                    },
                },
                spec: {
                    containers: [
                    {
                        name: 'main',
                        image: image,
                    },
                    ],
                },
            },
        },
    },
}


