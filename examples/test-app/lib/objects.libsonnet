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
    }
}


