local annotations = import './annotations.libsonnet';

function (object) object + if std.extVar('qbec.io/cleanMode') == 'on' then {} else { metadata +: { annotations +: annotations } }
