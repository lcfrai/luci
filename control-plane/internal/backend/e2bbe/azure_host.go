package e2bbe

import (
	"context"
	"net/http"
)

// DiscoverAzureHost is controller-only bootstrap admission. It cannot create a
// sandbox, and refuses a shared gateway returning more than one ready node.
func DiscoverAzureHost(ctx context.Context, config Config) (string, error) {
	if config.Deployment.AzurePrivateIP == "" || !ValidAzureDeployment(config.Deployment, config.Endpoint) || config.RootCAs == nil || config.ClientCertificate == nil || config.AdminAPIKey == "" {
		return "", ErrUnsupported
	}
	config.HostNodeID = "bootstrap"
	client, err := newClient(config)
	if err != nil {
		return "", err
	}
	defer client.http.CloseIdleConnections()
	controller := &Controller{config: config, client: client}
	if controller.VerifyGatewayReceipt(ctx) != nil {
		return "", ErrUnsupported
	}
	var nodes []providerNode
	status, _, err := client.callWithKey(ctx, "X-Admin-Token", client.adminKey, http.MethodGet, "/nodes", nil, nil, &nodes)
	if err != nil || status != http.StatusOK || len(nodes) != 1 || !validID(nodes[0].ID) || nodes[0].Status != "ready" || nodes[0].SandboxCount == nil || nodes[0].SandboxStartingCount == nil || *nodes[0].SandboxCount != 0 || *nodes[0].SandboxStartingCount != 0 {
		return "", ErrUnknown
	}
	return nodes[0].ID, nil
}
