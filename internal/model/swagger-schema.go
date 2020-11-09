package model

// generated by gen-qbec-swagger from swagger.yaml at 2020-11-09 11:44:20.253614 +0000 UTC
// Do NOT edit this file by hand

var swaggerJSON = `
{
    "definitions": {
        "qbec.io.v1alpha1.App": {
            "additionalProperties": false,
            "description": "The list of all components for the app is derived as all the supported (jsonnet, json, yaml) files in the components subdirectory.",
            "properties": {
                "apiVersion": {
                    "description": "requested API version",
                    "type": "string"
                },
                "kind": {
                    "description": "object kind",
                    "pattern": "^App$",
                    "type": "string"
                },
                "metadata": {
                    "$ref": "#/definitions/qbec.io.v1alpha1.AppMeta"
                },
                "spec": {
                    "$ref": "#/definitions/qbec.io.v1alpha1.AppSpec"
                }
            },
            "required": [
                "kind",
                "apiVersion",
                "metadata",
                "spec"
            ],
            "title": "QbecApp is a set of components that can be applied to multiple environments with tweaked runtime configurations.",
            "type": "object"
        },
        "qbec.io.v1alpha1.AppMeta": {
            "additionalProperties": false,
            "properties": {
                "name": {
                    "pattern": "^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$",
                    "type": "string"
                }
            },
            "required": [
                "name"
            ],
            "title": "AppMeta is the simplified metadata object for a qbec app.",
            "type": "object"
        },
        "qbec.io.v1alpha1.AppSpec": {
            "additionalProperties": false,
            "properties": {
                "baseProperties": {
                    "description": "properties for the baseline environment",
                    "type": "object"
                },
                "clusterScopedLists": {
                    "description": "whether remote lists should use cluster scoped queries when multiple namespaces present",
                    "type": "boolean"
                },
                "componentsDir": {
                    "description": "directory containing component files, default to components/",
                    "type": "string"
                },
                "envFiles": {
                    "description": "list of additional files containing environment definitions to load.\nenvironment definitions are merged in the order specified starting with any inline environments.\nAn environment defined in a later file takes precedence over the the same environment already loaded\nand replaces it.",
                    "items": {
                        "type": "string"
                    },
                    "type": "array"
                },
                "environments": {
                    "additionalProperties": {
                        "$ref": "#/definitions/qbec.io.v1alpha1.Environment"
                    },
                    "description": "set of environments for the app",
                    "minProperties": 1,
                    "type": "object"
                },
                "excludes": {
                    "description": "list of components to exclude by default for every environment",
                    "items": {
                        "type": "string"
                    },
                    "type": "array"
                },
                "libPaths": {
                    "description": "list of library paths to add to the jsonnet VM at evaluation",
                    "items": {
                        "type": "string"
                    },
                    "type": "array"
                },
                "namespaceTagSuffix": {
                    "description": "suffix default namespace when app-tag provided, with the supplied tag",
                    "type": "boolean"
                },
                "paramsFile": {
                    "description": "standard file containing parameters for all environments returning correct values based on qbec.io/env external\nvariable, defaults to params.libsonnet",
                    "type": "string"
                },
                "postProcessor": {
                    "description": "file containing jsonnet code that can be used to post-process all objects, typically adding metadata like\nannotations",
                    "type": "string"
                },
                "vars": {
                    "$ref": "#/definitions/qbec.io.v1alpha1.Variables"
                }
            },
            "title": "AppSpec is the user-supplied configuration of the qbec app.",
            "type": "object"
        },
        "qbec.io.v1alpha1.Environment": {
            "additionalProperties": false,
            "properties": {
                "context": {
                    "type": "string"
                },
                "defaultNamespace": {
                    "type": "string"
                },
                "excludes": {
                    "items": {
                        "type": "string"
                    },
                    "type": "array"
                },
                "includes": {
                    "items": {
                        "type": "string"
                    },
                    "type": "array"
                },
                "properties": {
                    "description": "open-ended object containing additional environment properties.",
                    "type": "object"
                },
                "server": {
                    "type": "string"
                }
            },
            "title": "Environment points to a specific destination and has its own set of runtime parameters.",
            "type": "object"
        },
        "qbec.io.v1alpha1.EnvironmentMap": {
            "additionalProperties": false,
            "properties": {
                "apiVersion": {
                    "description": "requested API version",
                    "type": "string"
                },
                "kind": {
                    "description": "object kind",
                    "pattern": "^EnvironmentMap$",
                    "type": "string"
                },
                "spec": {
                    "$ref": "#/definitions/qbec.io.v1alpha1.EnvironmentsSpec"
                }
            },
            "required": [
                "kind",
                "apiVersion",
                "spec"
            ]
        },
        "qbec.io.v1alpha1.EnvironmentsSpec": {
            "additionalProperties": false,
            "properties": {
                "environments": {
                    "additionalProperties": {
                        "$ref": "#/definitions/qbec.io.v1alpha1.Environment"
                    },
                    "description": "set of environments for the app",
                    "minProperties": 1,
                    "type": "object"
                }
            },
            "required": [
                "environments"
            ]
        },
        "qbec.io.v1alpha1.ExternalVar": {
            "additionalProperties": false,
            "properties": {
                "default": {
                    "nullable": true
                },
                "name": {
                    "type": "string"
                },
                "secret": {
                    "type": "boolean"
                }
            },
            "required": [
                "name"
            ],
            "title": "ExternalVar is a variable that is set as an extVar in the jsonnet VM",
            "type": "object"
        },
        "qbec.io.v1alpha1.TopLevelVar": {
            "additionalProperties": false,
            "properties": {
                "components": {
                    "items": {
                        "type": "string"
                    },
                    "minItems": 1,
                    "type": "array"
                },
                "name": {
                    "type": "string"
                },
                "secret": {
                    "type": "boolean"
                }
            },
            "required": [
                "name",
                "components"
            ],
            "title": "TopLevelVar is a variable that is set as a TLA in the jsonnet VM. Note that there is no provision to set\na default value - default values should be set in the jsonnet code instead.",
            "type": "object"
        },
        "qbec.io.v1alpha1.Variables": {
            "additionalProperties": false,
            "properties": {
                "external": {
                    "items": {
                        "$ref": "#/definitions/qbec.io.v1alpha1.ExternalVar"
                    },
                    "type": "array"
                },
                "topLevel": {
                    "items": {
                        "$ref": "#/definitions/qbec.io.v1alpha1.TopLevelVar"
                    },
                    "type": "array"
                }
            },
            "title": "Variables is a collection of external and top-level variables.",
            "type": "object"
        }
    },
    "paths": {},
    "swagger": "2.0"
}
`
