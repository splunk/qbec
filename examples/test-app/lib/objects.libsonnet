{
    configmap(namespace, name, vars={}):: {
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: {
            namespace: namespace,
            name: name,
        },
        data: vars,
    },
    secret(namespace,name,vars={}):: {
        apiVersion: "v1",
        kind: "Secret",
        metadata: {
            namespace: namespace,
            name: name,
        },
        data: vars,
    },
    deployment(namespace, name, image):: {
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        metadata: {
            namespace: namespace,
            name: name,
        },
        spec: {
            selector: {
                matchLabels: {
                    app: name
                },
            },
            template: {
                metadata: {
                    labels: { app: name },
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


