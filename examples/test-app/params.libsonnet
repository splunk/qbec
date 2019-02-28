local p = {
    _: import './environments/base.libsonnet',
    dev: import './environments/dev.libsonnet',
    prod: import './environments/prod.libsonnet',
};

local env = std.extVar('qbec.io/env');

if std.objectHas(p, env) then p[env] else error 'Environment ' + env + ' not defined in ' + std.thisFile

