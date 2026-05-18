package api

import "errors"

// ErrBuildURLUnsupported is returned by URL builders when the requested
// mode cannot be satisfied (for example Balancer-only routing where the
// provider exposes no matching venues for that chain).
var ErrBuildURLUnsupported = errors.New("unsupported")
