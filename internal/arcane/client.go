// Package arcane builds clients for the Arcane API (getarcane.app). Arcane
// answers JSON like Portainer but authenticates with an X-API-Key header, so it
// reuses the request executor of internal/portainer and only changes the header
// and the label errors carry.
package arcane

import (
	"github.com/akrhin/portainer-mcp/internal/config"
	"github.com/akrhin/portainer-mcp/internal/portainer"
)

// New builds a client for one configured Arcane manager.
func New(inst config.Instance) (*portainer.Client, error) {
	return portainer.New(portainer.Options{
		BaseURL:    inst.URL,
		APIKey:     inst.APIKey,
		TLSVerify:  bool(inst.TLSVerify),
		CAFile:     inst.CAFile,
		AuthHeader: "X-API-Key",
		Label:      "arcane",
	})
}
