//go:build sidecars

package sidecars

import _ "embed"

//go:embed embedded/vaultline
var vaultlineBinary []byte

var embeddedTools = map[string]Tool{
	"vaultline": {
		Name: "vaultline",
		Data: vaultlineBinary,
	},
}
