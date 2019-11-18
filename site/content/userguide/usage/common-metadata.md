---
title: Common object metadata
weight: 18
---

qbec provides an easy mechanism to set up common metadata like annotations for all objects
produced. For example, you may want to set up a `team` annotation for all objects.

You do this by defining a post-processor. A post-processor is a jsonnet file that contains
a single function like so:

```
// the post processor jsonnet must return a function taking exactly one parameter
// called "object" and returning its decorated version.

function (object) object { 
    metadata +: { 
        annotations +: { 
            team: 'my-team',
        },
    },
}
```

Note that the argument of the function *must* be called `object`.

You then set the `postProcessor` attribute in `qbec.yaml` set to the path of this file.

**Note:** It is possible to abuse this feature to do a lot more than adding metadata since it
is a hook that allows you to do almost anything to the supplied object. Abuse with care :)
