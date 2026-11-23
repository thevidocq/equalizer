package assistant

import (
	"context"
	"encoding/json"
	"strings"
)

const maxTokens = 2000

// anthropic speaks the Messages API, with tool use.
type anthropic struct{ c Config }

func (a *anthropic) Chat(ctx context.Context, system string, turns []Turn, tools []Tool) (Reply, error) {
	type block = map[string]any
	var msgs []map[string]any
	for _, t := range turns {
		switch t.Role {
		case "user":
			msgs = append(msgs, map[string]any{"role": "user", "content": t.Text})
		case "assistant":
			content := []block{}
			if t.Text != "" {
				content = append(content, block{"type": "text", "text": t.Text})
			}
			for _, c := range t.Calls {
				content = append(content, block{"type": "tool_use", "id": c.ID, "name": c.Name, "input": inputOf(c.Input)})
			}
			msgs = append(msgs, map[string]any{"role": "assistant", "content": content})
		case "tool":
			content := []block{}
			for _, r := range t.Results {
				content = append(content, block{"type": "tool_result", "tool_use_id": r.ID, "content": r.Content})
			}
			msgs = append(msgs, map[string]any{"role": "user", "content": content})
		}
	}
	var defs []map[string]any
	for _, t := range tools {
		defs = append(defs, map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Schema})
	}
	body := map[string]any{"model": a.c.Model, "max_tokens": maxTokens, "system": system, "messages": msgs}
	if len(defs) > 0 {
		body["tools"] = defs
	} else if hasTools(turns) {
		// the last round asks for words only; the history still holds tool
		// blocks, which the API reads only with tools defined
		body["tools"], body["tool_choice"] = toolDefsOf(turns), map[string]any{"type": "none"}
	}
	var out struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	err := post(ctx, a.c.Client, strings.TrimSuffix(or(a.c.AnthropicURL, "https://api.anthropic.com"), "/")+"/v1/messages",
		map[string]string{"x-api-key": a.c.Key, "anthropic-version": "2023-06-01"},
		body, &out)
	if err != nil {
		return Reply{}, err
	}
	var r Reply
	for _, b := range out.Content {
		switch b.Type {
		case "text":
			r.Text += b.Text
		case "tool_use":
			r.Calls = append(r.Calls, Call{ID: b.ID, Name: b.Name, Input: b.Input})
		}
	}
	return r, nil
}

// openai speaks the Chat Completions API (OpenAI, or a compatible server).
type openai struct {
	c    Config
	base string
}

func (o *openai) Chat(ctx context.Context, system string, turns []Turn, tools []Tool) (Reply, error) {
	msgs := []map[string]any{{"role": "system", "content": system}}
	for _, t := range turns {
		switch t.Role {
		case "user":
			msgs = append(msgs, map[string]any{"role": "user", "content": t.Text})
		case "assistant":
			m := map[string]any{"role": "assistant", "content": t.Text}
			var calls []map[string]any
			for _, c := range t.Calls {
				calls = append(calls, map[string]any{"id": c.ID, "type": "function",
					"function": map[string]any{"name": c.Name, "arguments": string(orJSON(c.Input))}})
			}
			if len(calls) > 0 {
				m["tool_calls"] = calls
			}
			msgs = append(msgs, m)
		case "tool":
			for _, r := range t.Results {
				msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": r.ID, "content": r.Content})
			}
		}
	}
	var defs []map[string]any
	for _, t := range tools {
		defs = append(defs, map[string]any{"type": "function", "function": map[string]any{
			"name": t.Name, "description": t.Description, "parameters": t.Schema}})
	}
	headers := map[string]string{}
	if o.c.Key != "" {
		headers["Authorization"] = "Bearer " + o.c.Key
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	body := map[string]any{"model": o.c.Model, "messages": msgs, "max_tokens": maxTokens}
	if len(defs) > 0 {
		body["tools"] = defs
	}
	err := post(ctx, o.c.Client, o.base+"/chat/completions", headers, body, &out)
	if err != nil {
		return Reply{}, err
	}
	if len(out.Choices) == 0 {
		return Reply{}, &ErrProvider{"unexpected answer from the AI provider"}
	}
	m := out.Choices[0].Message
	r := Reply{Text: m.Content}
	for _, c := range m.ToolCalls {
		r.Calls = append(r.Calls, Call{ID: c.ID, Name: c.Function.Name, Input: json.RawMessage(orJSON([]byte(c.Function.Arguments)))})
	}
	return r, nil
}

func hasTools(turns []Turn) bool {
	for _, t := range turns {
		if len(t.Calls) > 0 {
			return true
		}
	}
	return false
}

// toolDefsOf declares the tools the history used, for a words-only round.
func toolDefsOf(turns []Turn) []map[string]any {
	seen := map[string]bool{}
	var defs []map[string]any
	for _, t := range Tools() {
		for _, turn := range turns {
			for _, c := range turn.Calls {
				if c.Name == t.Name && !seen[t.Name] {
					seen[t.Name] = true
					defs = append(defs, map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Schema})
				}
			}
		}
	}
	if len(defs) == 0 { // a name the model made up: any tool will do
		t := Tools()[0]
		defs = append(defs, map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Schema})
	}
	return defs
}

func inputOf(raw json.RawMessage) any {
	var v map[string]any
	if json.Unmarshal(raw, &v) != nil || v == nil {
		return map[string]any{}
	}
	return v
}

func orJSON(raw []byte) []byte {
	if !json.Valid(raw) {
		return []byte("{}")
	}
	return raw
}
