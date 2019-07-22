// the post processor jsonnet must return a function taking exactly one parameter
// called "object" and returning its decorated version.

function (object) object { metadata +: { annotations +: { slack: '#my-channel' } } }

