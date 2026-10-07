package api

import (
	"context"
	"errors"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// resolveAssistanceConnection shares the same route used by coding sessions.
// Only legacy clients may specify an auth mode. An explicit connection never
// falls back to a different account, even when that account is already saved.
func (a *API) resolveAssistanceConnection(ctx context.Context, harness, model, legacyAuth string, selection *string, explicitModel bool) (product.AgentConnectionReceipt, map[string]string, *codexConnection, error) {
	var receipt product.AgentConnectionReceipt
	if harness != "codex" && harness != "claude-code" && harness != "opencode" {
		return receipt, nil, nil, errors.New("Choose Codex, Claude Code or OpenCode for the assistant.")
	}
	if legacyAuth != "" {
		if selection != nil && *selection != "" {
			return receipt, nil, nil, errors.New("Choose one connection; a legacy sign-in mode cannot override it.")
		}
		// Preserve the explicit provider choice made by older repository clients.
		empty := ""
		selection = &empty
	}
	receipt, env, err := a.deps.Product.ResolveAgentConnection(harness, selection)
	if err != nil {
		return receipt, nil, nil, err
	}
	if env == nil {
		env = map[string]string{}
	}
	if receipt.ID != "" && model != "" && model != "default" && model != receipt.Model && !(harness == "opencode" && model == "luci-gateway/"+receipt.Model) {
		return receipt, nil, nil, errors.New("The selected connection has a different saved model. Refresh the connection and start with its configured model.")
	}
	auth := receipt.AuthMode
	var account *codexConnection
	if harness == "codex" {
		// A configured API route takes precedence over the saved subscription.
		// An unselected legacy route retains Codex's subscription-first behavior.
		legacy := receipt.ID == "" || receipt.Kind == "legacy" || strings.HasPrefix(receipt.ID, "legacy-")
		wantSubscription := auth == "chatgpt" || legacyAuth == "chatgpt" || legacy && legacyAuth == ""
		if wantSubscription {
			account, _, err = a.selectCodexConnection(ctx, registry.Session{}, false, "")
			if err != nil {
				return receipt, nil, nil, err
			}
			if account != nil {
				auth, env = "chatgpt", map[string]string{}
			} else if receipt.Kind == "codex-subscription" || legacyAuth == "chatgpt" || auth == "chatgpt" {
				return receipt, nil, nil, errCodexDisconnected
			}
		}
		if account == nil {
			auth = "api"
			if env["OPENAI_API_KEY"] == "" {
				return receipt, nil, nil, errors.New("Connect a Codex subscription or a Responses API connection before using this assistant.")
			}
		}
	} else if harness == "claude-code" {
		auth = "api"
		if env["ANTHROPIC_API_KEY"] == "" && env["ANTHROPIC_AUTH_TOKEN"] == "" {
			return receipt, nil, nil, errors.New("Connect a Claude Code Messages API connection before using this assistant.")
		}
	} else {
		auth = "configured"
		if env["OPENCODE_CONFIG_CONTENT"] == "" {
			return receipt, nil, nil, errors.New("Connect an OpenCode compatible API before using this assistant.")
		}
	}
	if model == "" || model == "default" {
		model = receipt.Model
	}
	if model == "" && !explicitModel && harness == "codex" {
		// Codex can use its own current default for a legacy interview request.
		model = "default"
	}
	if !(model == "default" && !explicitModel && harness == "codex") {
		if err := assistance.ValidateSelection(harness, model, auth); err != nil {
			return receipt, nil, nil, err
		}
	}
	if legacyAuth != "" && legacyAuth != auth {
		return receipt, nil, nil, errors.New("The selected connection does not support that provider mode.")
	}
	receipt.AuthMode, receipt.Model = auth, model
	if harness == "opencode" {
		if err := assistance.ReadOnlyOpenCode(env, model); err != nil {
			return receipt, nil, nil, err
		}
		receipt.Model = env["LUCI_OPENCODE_MODEL"]
	}
	return receipt, env, account, nil
}

func bindAssistantConnection(session *registry.Session, receipt product.AgentConnectionReceipt) {
	session.AgentConnectionID = receipt.ID
	session.AgentConnectionRevision = receipt.Revision
	session.AgentConnectionKind = receipt.Kind
	session.AgentConnectionProtocol = receipt.Protocol
}

func bindAssistantJob(job *assistance.Job, receipt product.AgentConnectionReceipt) {
	job.AgentConnectionID = receipt.ID
	job.AgentConnectionRevision = receipt.Revision
	job.AgentConnectionKind = receipt.Kind
	job.AgentConnectionProtocol = receipt.Protocol
}
