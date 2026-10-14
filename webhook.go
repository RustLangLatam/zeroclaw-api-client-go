package zeroclaw

// One-shot messages to an agent: the generic webhook and the A2A
// (agent-to-agent) JSON-RPC endpoint.
//
// Gateway handlers: crates/zeroclaw-gateway/src/lib.rs (handle_webhook) and
// a2a.rs. Both run a whole agent turn and answer with its reply; for
// streaming, tool approvals or a lasting conversation use [Client.Chat].

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// WebhookAPI holds the webhook and A2A endpoints, from [Client.Webhook].
type WebhookAPI struct{ c *Client }

// Webhook returns the generic webhook and A2A endpoints.
func (c *Client) Webhook() WebhookAPI { return WebhookAPI{c} }

// WebhookOptions holds the optional parameters of [WebhookAPI.Send]; empty
// ones are not sent.
type WebhookOptions struct {
	// Agent is the agent to answer (the default agent when empty).
	Agent string
	// Secret is sent as X-Webhook-Secret, when the gateway sets
	// webhook_secret.
	Secret string
	// IdempotencyKey is sent as X-Idempotency-Key: a repeat within the
	// gateway's window is not run again.
	IdempotencyKey string
	// SessionID is sent as X-Session-Id: it keeps calls in one conversation
	// ([A-Za-z0-9._-], at most 128 characters).
	SessionID string
}

// Send sends message and returns the agent's reply (POST /webhook). A
// duplicate idempotency key answers a [WebhookReply] with Duplicate set.
func (a WebhookAPI) Send(ctx context.Context, message string, opts WebhookOptions) (*WebhookReply, error) {
	req, err := a.c.jsonRequest(ctx, http.MethodPost, []string{"webhook"}, params("agent", opts.Agent), map[string]string{"message": message})
	if err != nil {
		return nil, err
	}
	for header, value := range map[string]string{
		"X-Webhook-Secret":  opts.Secret,
		"X-Idempotency-Key": opts.IdempotencyKey,
		"X-Session-Id":      opts.SessionID,
	} {
		if value != "" {
			req.Header.Set(header, value)
		}
	}
	var raw json.RawMessage
	if err := a.c.do(req, &raw); err != nil {
		return nil, err
	}
	var probe struct {
		Status any `json:"status"`
	}
	if err := decode(raw, &probe); err != nil {
		return nil, err
	}
	if probe.Status == "duplicate" {
		return &WebhookReply{Duplicate: true}, nil
	}
	var reply AgentReply
	if err := decode(raw, &reply); err != nil {
		return nil, err
	}
	return &WebhookReply{Reply: &reply}, nil
}

// A2ASend sends text to a published agent with message/send and returns
// the text of its answer (POST /a2a/{alias}). A JSON-RPC error is returned
// as an [*APIError] with status 200. Needs the a2a feature and
// a2a.server.enabled on the gateway.
func (a WebhookAPI) A2ASend(ctx context.Context, alias, text string) (string, error) {
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "message/send",
		"params": map[string]any{"message": map[string]any{
			"parts": []map[string]string{{"kind": "text", "text": text}},
		}},
	}
	var raw json.RawMessage
	if err := a.c.call(ctx, http.MethodPost, []string{"a2a", alias}, nil, body, &raw); err != nil {
		return "", err
	}
	var answer struct {
		Error *struct {
			Message any `json:"message"`
		} `json:"error"`
		Result struct {
			Artifacts []struct {
				Parts []struct {
					Text any `json:"text"`
				} `json:"parts"`
			} `json:"artifacts"`
		} `json:"result"`
	}
	if err := decode(raw, &answer); err != nil {
		return "", err
	}
	if answer.Error != nil {
		message, ok := answer.Error.Message.(string)
		if !ok {
			message = "A2A error"
		}
		return "", &APIError{Status: http.StatusOK, Message: message, Body: raw}
	}
	var texts []string
	for _, artifact := range answer.Result.Artifacts {
		for _, part := range artifact.Parts {
			if t, ok := part.Text.(string); ok {
				texts = append(texts, t)
			}
		}
	}
	return strings.Join(texts, "\n"), nil
}

// AgentCard returns a published agent's A2A card (GET
// /a2a/{alias}/.well-known/agent-card.json). Public.
func (a WebhookAPI) AgentCard(ctx context.Context, alias string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"a2a", alias, ".well-known", "agent-card.json"}, nil, &out)
}

// WebhookReply is the answer of [WebhookAPI.Send]: the agent's reply, or
// Duplicate when the idempotency key was already used and nothing ran.
type WebhookReply struct {
	// Reply is the agent's reply; nil when Duplicate.
	Reply *AgentReply
	// Duplicate is whether the idempotency key was already used.
	Duplicate bool
}

// AgentReply is an agent's reply to a webhook.
type AgentReply struct {
	// Response is the reply.
	Response string `json:"response"`
	// Model is the model that answered.
	Model string `json:"model"`
}
