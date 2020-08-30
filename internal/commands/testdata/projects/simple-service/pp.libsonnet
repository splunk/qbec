function(object) (
  // please don't do things like this for real. This is just to make my testing easy :)
  local replicas = std.extVar('replicas');
  local cmContent = std.extVar('cmContent');
  if object.kind == 'Deployment' then
    object { spec+: { replicas: replicas } }
  else if object.kind == 'ConfigMap' then
    object { data+: { 'index.html': cmContent } }
  else
    object
)
