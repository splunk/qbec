local annotations = import './annotations.libsonnet';

function (object) object  { metadata +: { annotations +: annotations } }
