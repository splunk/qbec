import "qbec.io/helper"
#fooVal: *"bar" | string @tag(fooVal)
apiVersion: "v1"
kind: "ConfigMap"
metadata: {
  name: helper.name
  annotations: {
    "qbec.io/injectedVar": #fooVal
    "qbec.io/injectedBuiltinOS": string @tag(someTagname,var=os)
  }
}
data: foo: #fooVal
