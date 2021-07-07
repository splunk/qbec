// we pull in the generated config map as a string usiong importstr
// the syntax for the import is:
//   data://<data-source-name>[/optional/path]
// in this data source implementation the path appears in the config map
// when used for real it can contain information such as the path to a vault KV entry,
// a path to a helm chart and so on.
local cmYaml = importstr 'data://config-map/some/path';
std.native('parseYaml')(cmYaml)
