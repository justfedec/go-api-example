package stdlib

import "github.com/justfedec/inkdown/internal/types"

// The llm namespace talks to the Anthropic Messages API with nothing but the
// Go standard library, preserving Inkdown's zero-dependency builds. llm.ask
// panics with a self-describing message on any failure (missing key, HTTP
// error, refusal) — scripts get the two-line happy path, and programs that
// need recoverable calls can drive the API by hand with http.request + json.
//
// Environment: ANTHROPIC_API_KEY (required), INKDOWN_LLM_MODEL (default
// claude-opus-4-8), INKDOWN_LLM_MAX_TOKENS (default 16000), and
// ANTHROPIC_BASE_URL (default https://api.anthropic.com — overriding it makes
// llm.ask testable against a local fake server).
func init() {
	Roots["llm"] = true

	register(
		&Spec{Name: "llm.ask", Params: []types.Type{types.String, types.String}, MinArgs: 1, Ret: types.String, GoFunc: "_ink_llmAsk", GoFuncErr: "_ink_llmAskErr", Chunk: "llm"},
	)

	Chunks["llm"] = &Chunk{
		Imports: []string{"bytes", "encoding/json", "io", "net/http", "os", "time"},
		Src: `var _ink_llmClient = &http.Client{Timeout: 10 * time.Minute}

// _ink_llmAsk is the panic-on-failure form; _ink_llmAskErr below is the
// recoverable form for 'let reply, err = llm.ask(...)'.
func _ink_llmAsk(prompt string, system ...string) string {
	reply, msg := _ink_llmAskErr(prompt, system...)
	if msg != "" {
		panic(msg)
	}
	return reply
}

func _ink_llmAskErr(prompt string, system ...string) (string, string) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", "llm.ask: set the ANTHROPIC_API_KEY environment variable"
	}
	model := os.Getenv("INKDOWN_LLM_MODEL")
	if model == "" {
		model = "claude-opus-4-8"
	}
	maxTokens := 16000
	if s := os.Getenv("INKDOWN_LLM_MAX_TOKENS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			maxTokens = n
		}
	}
	base := os.Getenv("ANTHROPIC_BASE_URL")
	if base == "" {
		base = "https://api.anthropic.com"
	}

	// The thinking parameter is omitted on purpose: every current model has
	// a sensible default, and older overrides via INKDOWN_LLM_MODEL would
	// reject an explicit configuration.
	reqBody := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"messages": []any{
			map[string]any{"role": "user", "content": prompt},
		},
	}
	if len(system) > 0 && system[0] != "" {
		reqBody["system"] = system[0]
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", "llm.ask: " + err.Error()
	}

	req, err := http.NewRequest("POST", base+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", "llm.ask: " + err.Error()
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := _ink_llmClient.Do(req)
	if err != nil {
		return "", "llm.ask: " + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "llm.ask: the API returned HTTP " + strconv.Itoa(resp.StatusCode) + ": " + string(body)
	}

	var out struct {
		StopReason string ` + "`json:\"stop_reason\"`" + `
		Content    []struct {
			Type string ` + "`json:\"type\"`" + `
			Text string ` + "`json:\"text\"`" + `
		} ` + "`json:\"content\"`" + `
		StopDetails *struct {
			Explanation string ` + "`json:\"explanation\"`" + `
		} ` + "`json:\"stop_details\"`" + `
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "llm.ask: cannot parse the API response: " + err.Error()
	}
	if out.StopReason == "refusal" {
		msg := "llm.ask: the model declined this request"
		if out.StopDetails != nil && out.StopDetails.Explanation != "" {
			msg += ": " + out.StopDetails.Explanation
		}
		return "", msg
	}
	if out.StopReason == "max_tokens" {
		return "", "llm.ask: the reply was cut off by the token budget; raise INKDOWN_LLM_MAX_TOKENS (default 16000)"
	}
	for _, block := range out.Content {
		if block.Type == "text" {
			return block.Text, ""
		}
	}
	return "", "llm.ask: the response contains no text"
}
`,
	}
}
