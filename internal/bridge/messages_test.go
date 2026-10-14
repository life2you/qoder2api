package bridge

import (
	"testing"
)

func TestDeveloperRoleMapping(t *testing.T) {
	// 1. 测试 ConvertIncomingMessage 将 developer 转换为 system
	devMsg := map[string]interface{}{
		"role":    "developer",
		"content": "You are a helpful assistant.",
	}
	converted := ConvertIncomingMessage(devMsg, false)
	if converted == nil {
		t.Fatalf("expected converted message, got nil")
	}
	if role, ok := converted["role"].(string); !ok || role != "system" {
		t.Errorf("expected role system, got %v", converted["role"])
	}
	if content, ok := converted["content"].(string); !ok || content != "You are a helpful assistant." {
		t.Errorf("expected content 'You are a helpful assistant.', got %v", converted["content"])
	}

	// 2. 测试 BuildQoderMessages 识别 developer 为 system，防止注入 template system message
	templateMsgs := []interface{}{
		map[string]interface{}{"role": "system", "content": "template system prompt"},
	}
	incoming := []interface{}{
		devMsg,
		map[string]interface{}{"role": "user", "content": "hello"},
	}
	rebuilt := BuildQoderMessages(templateMsgs, incoming, "hello", false)
	if len(rebuilt) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d: %+v", len(rebuilt), rebuilt)
	}
	first, _ := rebuilt[0].(map[string]interface{})
	if first["role"] != "system" || first["content"] != "You are a helpful assistant." {
		t.Errorf("expected first message to be user-provided system message, got %+v", first)
	}

	// 3. 测试 CodexInputToMessages 将 developer 转换为 system
	codexInput := []interface{}{
		map[string]interface{}{
			"type":    "message",
			"role":    "developer",
			"content": "Developer instructions",
		},
	}
	codexMsgs := CodexInputToMessages(codexInput, "")
	if len(codexMsgs) != 1 {
		t.Fatalf("expected 1 codex message, got %d", len(codexMsgs))
	}
	cMsg, _ := codexMsgs[0].(map[string]interface{})
	if cMsg["role"] != "system" {
		t.Errorf("expected role system, got %v", cMsg["role"])
	}
}

func TestNormalizeTools(t *testing.T) {
	// 1. 扁平 Codex 格式（带 type: function，但缺少 function 包裹）
	flatCodexTools := []interface{}{
		map[string]interface{}{
			"type":        "function",
			"name":        "run_bash",
			"description": "Run shell command",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	norm1 := NormalizeTools(flatCodexTools)
	arr1, ok := norm1.([]interface{})
	if !ok || len(arr1) != 1 {
		t.Fatalf("expected 1 tool, got %v", norm1)
	}
	t1 := arr1[0].(map[string]interface{})
	if t1["type"] != "function" {
		t.Errorf("expected type function, got %v", t1["type"])
	}
	fn1, ok := t1["function"].(map[string]interface{})
	if !ok || fn1["name"] != "run_bash" || fn1["description"] != "Run shell command" {
		t.Errorf("unexpected function object: %+v", fn1)
	}

	// 2. Claude 格式（使用 input_schema）
	claudeTools := []interface{}{
		map[string]interface{}{
			"name":         "search",
			"description":  "Search web",
			"input_schema": map[string]interface{}{"type": "object"},
		},
	}
	norm2 := NormalizeTools(claudeTools)
	arr2 := norm2.([]interface{})
	t2 := arr2[0].(map[string]interface{})
	fn2 := t2["function"].(map[string]interface{})
	if fn2["name"] != "search" || fn2["parameters"] == nil {
		t.Errorf("unexpected function object for claude tool: %+v", fn2)
	}

	// 3. 标准 OpenAI 格式保持不变
	stdTools := []interface{}{
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "std_fn",
				"description": "std",
				"parameters":  map[string]interface{}{"type": "object"},
			},
		},
	}
	norm3 := NormalizeTools(stdTools)
	arr3 := norm3.([]interface{})
	t3 := arr3[0].(map[string]interface{})
	fn3 := t3["function"].(map[string]interface{})
	if fn3["name"] != "std_fn" {
		t.Errorf("unexpected std tool: %+v", fn3)
	}
}

func TestSanitizeToolCallsSequence(t *testing.T) {
	// 测试场景 1：assistant 包含 call_1 和 call_2，但客户端只回复了 call_1
	inputMsgs := []interface{}{
		map[string]interface{}{
			"role":    "assistant",
			"content": "thinking",
			"tool_calls": []interface{}{
				map[string]interface{}{"id": "call_1", "type": "function"},
				map[string]interface{}{"id": "call_2", "type": "function"},
			},
		},
		map[string]interface{}{
			"role":         "tool",
			"tool_call_id": "call_1",
			"content":      "result_1",
		},
	}

	sanitized := SanitizeToolCallsSequence(inputMsgs)
	// 应自动为 call_2 补齐 tool 响应
	if len(sanitized) != 3 {
		t.Fatalf("expected 3 messages after sanitize, got %d: %+v", len(sanitized), sanitized)
	}
	third, _ := sanitized[2].(map[string]interface{})
	if third["role"] != "tool" || third["tool_call_id"] != "call_2" {
		t.Errorf("expected synthesized tool response for call_2, got %+v", third)
	}

	// 测试场景 2：连续两个 assistant 消息包含 tool_calls，应被合并
	consecutiveMsgs := []interface{}{
		map[string]interface{}{
			"role": "assistant",
			"tool_calls": []interface{}{
				map[string]interface{}{"id": "call_a", "type": "function"},
			},
		},
		map[string]interface{}{
			"role": "assistant",
			"tool_calls": []interface{}{
				map[string]interface{}{"id": "call_b", "type": "function"},
			},
		},
		map[string]interface{}{
			"role":         "tool",
			"tool_call_id": "call_a",
			"content":      "res_a",
		},
		map[string]interface{}{
			"role":         "tool",
			"tool_call_id": "call_b",
			"content":      "res_b",
		},
	}
	merged := SanitizeToolCallsSequence(consecutiveMsgs)
	if len(merged) != 3 { // 1 个合并后的 assistant + 2 个 tool
		t.Fatalf("expected 3 messages after merge consecutive assistants, got %d", len(merged))
	}
	asst, _ := merged[0].(map[string]interface{})
	tcs, _ := asst["tool_calls"].([]interface{})
	if len(tcs) != 2 {
		t.Errorf("expected 2 merged tool_calls, got %d", len(tcs))
	}
}
