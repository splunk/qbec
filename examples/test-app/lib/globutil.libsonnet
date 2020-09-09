local len = std.length;
local split = std.split;
local join = std.join;

// keepDirs returns a key mapping function for the number of directories to be retained
local keepDirs = function(num=0) function(s) (
  if num < 0
  then
    s
  else (
    local elems = split(s, '/');
    local preserveRight = num + 1;
    if len(elems) <= preserveRight
    then
      s
    else (
      local remove = len(elems) - preserveRight;
      join('/', elems[remove:])
    )
  )
);

// stripExtension is a key mapping function that strips the file extension from the key
local stripExtension = function(s) (
  local parts = split(s, '/');
  local dirs = parts[:len(parts) - 1];
  local file = parts[len(parts) - 1];
  local fileParts = split(file, '.');
  local fixed = if len(fileParts) == 1 then file else join('.', fileParts[:len(fileParts) - 1]);
  join('/', dirs + [fixed])
);

// compose composes an array of map functions by applying them in sequence
local compose = function(arr) function(s) std.foldl(function(prev, fn) fn(prev), arr, s);

// transform transforms an object, mapping keys using the key mapper and values using the valueMapper.
// It ensures that the key mapping does not produce duplicate keys.
local transform = function(globObject, keyMapper=function(s) s, valueMapper=function(o) o) (
  local keys = std.objectFields(globObject);
  std.foldl(function(obj, key) (
    local mKey = keyMapper(key);
    local val = globObject[key];
    if std.objectHas(obj, mKey)
    then
      error 'multiple keys map to the same value: %s' % [mKey]
    else
      obj { [mKey]: valueMapper(val) }
  ), keys, {})
);

// nameOnly is a key mapper that removes all directories and strips extensions from file names,
// syntax sugar for the common case.
local nameOnly = compose([keepDirs(0), stripExtension]);

{
  transform:: transform,
  keepDirs:: keepDirs,
  stripExtension:: stripExtension,
  compose:: compose,
  nameOnly:: nameOnly,
}
