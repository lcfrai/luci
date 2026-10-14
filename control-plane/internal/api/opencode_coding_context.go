package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// A local Ollama alias has a serving window, which is distinct from the
// model's trained window. OpenCode's custom-provider default is zero; with a
// zero window its overflow check never compacts a coding conversation.
func prepareLocalOpenCodeCodingContext(ctx context.Context, env map[string]string, selectedEngine backend.Backend) error {
	return prepareLocalOpenCodeCodingContextWith(ctx, env, func(ctx context.Context) error {
		fencer, ok := selectedEngine.(backend.LaunchFencer)
		if !ok {
			return errors.New("The selected Docker engine cannot verify this local Ollama connection.")
		}
		if _, err := fencer.LaunchFenceIdentity(ctx); err != nil {
			return errors.New("The selected Docker engine cannot verify this local Ollama connection. Choose a local Docker engine or a supported remote model route.")
		}
		return nil
	}, readLocalOllamaContext)
}

func prepareLocalOpenCodeCodingContextWith(ctx context.Context, env map[string]string, localEngine func(context.Context) error, readContext func(context.Context, string, string) (int, error)) error {
	var config map[string]any
	if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil {
		return nil // The existing connection validator owns malformed configs.
	}
	model, _ := config["model"].(string)
	alias, ok := strings.CutPrefix(model, "luci-local/")
	if !ok || alias == "" {
		return nil
	}
	providers, _ := config["provider"].(map[string]any)
	provider, _ := providers["luci-local"].(map[string]any)
	options, _ := provider["options"].(map[string]any)
	base, _ := options["baseURL"].(string)
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || u.Port() != "11434" || u.Path != "/v1" || u.RawQuery != "" || u.User != nil {
		return nil // Other compatible servers retain their configured contract.
	}
	host := u.Hostname()
	if host != "host.docker.internal" && host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil
	}
	models, _ := provider["models"].(map[string]any)
	entry, _ := models[alias].(map[string]any)
	if entry == nil {
		return errors.New("The saved OpenCode model is missing from its provider configuration.")
	}
	if err := localEngine(ctx); err != nil {
		return err
	}
	// The Docker host alias is meaningful inside the worker. The local API
	// observes the same host's loopback Ollama before dispatching the worker.
	probeHost := net.JoinHostPort("127.0.0.1", "11434")
	if host == "::1" {
		probeHost = net.JoinHostPort("::1", "11434")
	}
	window, err := readContext(ctx, probeHost, alias)
	if err != nil {
		return err
	}
	if window < 4096 || window > 131072 {
		return errors.New("The saved Ollama model needs an explicit supported serving context of at least 4,096 tokens before OpenCode can manage a coding conversation.")
	}
	output := window / 4
	if output > 2048 {
		output = 2048
	}
	entry["limit"] = map[string]int{"context": window, "output": output}
	encoded, err := json.Marshal(config)
	if err != nil {
		return errors.New("Could not prepare the saved OpenCode model context.")
	}
	env["OPENCODE_CONFIG_CONTENT"] = string(encoded)
	return nil
}

func clearReviewedOpenCodeInstructions(env map[string]string) error {
	var config map[string]any
	if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil {
		return errors.New("Could not prepare the reviewed OpenCode context.")
	}
	// The captured bundle is this run's authority. OpenCode's general
	// LUCI.md instruction would also load the mutable checkout's copy into
	// the model context, duplicating it and potentially making it stale.
	config["instructions"] = []string{}
	data, err := json.Marshal(config)
	if err != nil {
		return errors.New("Could not prepare the reviewed OpenCode context.")
	}
	env["OPENCODE_CONFIG_CONTENT"] = string(data)
	return nil
}

func readLocalOllamaContext(ctx context.Context, host, alias string) (int, error) {
	probe := &url.URL{Scheme: "http", Host: host, Path: "/api/show"}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probe.String(), strings.NewReader(fmt.Sprintf(`{"model":%q}`, alias)))
	if err != nil {
		return 0, errors.New("Could not verify the saved local model's serving context.")
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Transport:     &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, DisableKeepAlives: true},
		Timeout:       2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, errors.New("Could not verify the saved local model's serving context. Check that Ollama is running before starting OpenCode.")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, errors.New("Ollama did not confirm the exact saved model. Check the selected model before starting OpenCode.")
	}
	var shown struct {
		Parameters string `json:"parameters"`
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil || len(data) > 1<<20 || json.Unmarshal(data, &shown) != nil {
		return 0, errors.New("Could not verify the saved local model's serving context.")
	}
	window := 0
	for _, line := range strings.Split(shown.Parameters, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "num_ctx" {
			window, err = strconv.Atoi(fields[1])
			if err != nil {
				window = 0
			}
		}
	}
	return window, nil
}
