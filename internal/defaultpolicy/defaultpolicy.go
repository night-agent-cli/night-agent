package defaultpolicy

import _ "embed"

//go:embed configs/default_policy.yaml
var DefaultPolicyBytes []byte
