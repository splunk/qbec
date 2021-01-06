local globutil = import 'globutil.libsonnet';
local p = globutil.transform(import 'glob-import:environments/*.libsonnet', globutil.nameOnly);

local key = std.extVar('qbec.io/env');
if std.objectHas(p, key) then p[key] else error 'Environment ' + key + ' not defined in environments/'
