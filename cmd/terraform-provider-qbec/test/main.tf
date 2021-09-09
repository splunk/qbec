terraform {
  // from: https://learn.hashicorp.com/tutorials/terraform/provider-setup#test-the-provider
  //
  // do something like this to set up local testing from the qbec source root (on MacOs with binaries installed to ~/go/bin in my setup)
  //    go install ./... && cp ~/go/bin/terraform-provider-qbec  ~/.terraform.d/plugins/splunk.com/qbec/qbec/0.1.0/darwin_amd64/
  //
  // then from this test directory as the working directory, you can do:
  //    rm -f .terraform.lock.hcl && tf init && tf apply --auto-approve # where "tf" is terraform 0.14+
  required_providers {
    qbec = {
      version = "0.1.0"
      source  = "splunk.com/qbec/qbec"
    }
  }
}

data "qbec_eval" "basic" {
  root      = path.module
  lib_paths = ["lib"]
  file      = "main.jsonnet"
}

data "qbec_eval" "inline_code_and_vars" {
  root      = path.module
  lib_paths = ["lib"]
  ext_vars = {
    string1 = "ext-string1"
  }
  ext_code_vars = {
    array1 = "[ 1, 2, 3]"
  }
  tla_vars = {
    str = "tla-string1"
  }
  tla_code_vars = {
    numbers = "[1, 2, 3]"
  }
  code = <<EOT
    function (str, numbers) {
      foo: import 'foo.libsonnet',
      vars: {
        str: std.extVar('string1'),
        num: std.extVar('array1'),
      },
      tla_vars: {
        str: str,
        num: numbers,
      }
    }
  EOT
}

data "qbec_eval" "yaml_single" {
  root   = path.module
  format = "yaml"
  code   = <<EOT
  {
    foo: 'bar',
    bar: 'baz',
    quux: 'quux',
  }
  EOT
}

data "qbec_eval" "yaml_multi" {
  root   = path.module
  format = "multi-yaml"
  code   = <<EOT
  [
    {
      foo: 'bar',
      bar: 'baz',
      quux: 'quux',
    },
    {
      foo: 'ba2',
      bar: 'baz2',
      quux: 'quux2',
    },
  ]
  EOT
}

output "basic" {
  value = jsondecode(data.qbec_eval.basic.result)
}

output "inline_code_and_vars" {
  value = jsondecode(data.qbec_eval.inline_code_and_vars.result)
}

output "yaml_single" {
  value = data.qbec_eval.yaml_single.result
}

output "yaml_multi" {
  value = data.qbec_eval.yaml_multi.result
}
