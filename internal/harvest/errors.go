package harvest

import "errors"

// ErrNoRepos is returned when neither --repos nor a --projects-dir is
// configured: there is nothing to harvest. One sentinel (previously the
// same message was inlined in Run and Audit independently, so callers
// could not match it).
var ErrNoRepos = errors.New("harvest: no repos and no projects dir configured")
