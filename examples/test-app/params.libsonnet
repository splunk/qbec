local p = import 'glob:./environments/*.libsonnet?dirs=0&strip-extension=true';
local env = std.extVar('qbec.io/env');
local pEnv = if env == '_' then 'base' else env;

if std.objectHas(p, pEnv) then p[pEnv] else error 'Environment ' + env + ' not defined in ' + std.thisFile

