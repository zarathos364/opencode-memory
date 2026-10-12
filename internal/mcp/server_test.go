package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zarathos364/opencode-memory/internal/service"
)

// serve runs one session over the given lines and returns one reply per line.
func serve(t *testing.T, root string, lines ...string) []string {
	t.Helper()
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	out := &bytes.Buffer{}
	srv := &Server{Root: root, Version: "test"}
	if err := srv.Run(in, out); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	trimmed := strings.TrimSpace(out.String())
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func storeRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "opencode")
}

func decodeReply(t *testing.T, line string) map[string]any {
	t.Helper()
	var reply map[string]any
	if err := json.Unmarshal([]byte(line), &reply); err != nil {
		t.Fatalf("reply is not JSON: %v\n%s", err, line)
	}
	return reply
}

// resultText returns the text payload of a tools/call reply, failing the test
// when the reply carries isError.
func resultText(t *testing.T, line string) string {
	t.Helper()
	reply := decodeReply(t, line)
	result, ok := reply["result"].(map[string]any)
	if !ok {
		t.Fatalf("reply has no result:\n%s", line)
	}
	if flagged, _ := result["isError"].(bool); flagged {
		t.Fatalf("call returned an error:\n%s", line)
	}
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("call returned %d content parts, want 1:\n%s", len(content), line)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	return text
}

func errorText(t *testing.T, line string) string {
	t.Helper()
	reply := decodeReply(t, line)
	result, ok := reply["result"].(map[string]any)
	if !ok {
		t.Fatalf("reply has no result:\n%s", line)
	}
	flagged, _ := result["isError"].(bool)
	if !flagged {
		t.Fatalf("call missed isError:\n%s", line)
	}
	content, _ := result["content"].([]any)
	text, _ := content[0].(map[string]any)["text"].(string)
	return text
}

func requestLine(id any, method, params string) string {
	idText, _ := json.Marshal(id)
	if params == "" {
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":%q}`, idText, method)
	}
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":%q,"params":%s}`, idText, method, params)
}

func callLine(id any, name, args string) string {
	if args == "" {
		args = "{}"
	}
	return requestLine(id, "tools/call", fmt.Sprintf(`{"name":%q,"arguments":%s}`, name, args))
}

func TestInitializeCompletesTheHandshake(t *testing.T) {
	replies := serve(t, storeRoot(t),
		requestLine(1, "initialize", `{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"0"}}`),
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	)
	if len(replies) != 1 {
		t.Fatalf("session returned %d replies, want 1 (the notification gets none)", len(replies))
	}
	reply := decodeReply(t, replies[0])
	result, _ := reply["result"].(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Errorf("protocolVersion = %v, want %q", result["protocolVersion"], protocolVersion)
	}
	capabilities, _ := result["capabilities"].(map[string]any)
	if _, ok := capabilities["tools"]; !ok {
		t.Errorf("capabilities miss the tools key:\n%s", replies[0])
	}
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != "opencode-memory" {
		t.Errorf("server name = %v, want opencode-memory", info["name"])
	}
}

func TestMalformedInputGetsAnErrorObject(t *testing.T) {
	replies := serve(t, storeRoot(t), `this is not json`)
	if len(replies) != 1 {
		t.Fatalf("session returned %d replies, want 1", len(replies))
	}
	reply := decodeReply(t, replies[0])
	failure, ok := reply["error"].(map[string]any)
	if !ok {
		t.Fatalf("reply has no error object:\n%s", replies[0])
	}
	if failure["code"] != float64(errParse) {
		t.Errorf("code = %v, want %d", failure["code"], errParse)
	}
	if reply["id"] != nil {
		t.Errorf("id = %v, want null for an unparseable line", reply["id"])
	}
}

func TestUnknownMethodIsReported(t *testing.T) {
	replies := serve(t, storeRoot(t), requestLine(1, "tools/frobnicate", ""))
	reply := decodeReply(t, replies[0])
	failure, ok := reply["error"].(map[string]any)
	if !ok {
		t.Fatalf("reply has no error object:\n%s", replies[0])
	}
	if failure["code"] != float64(errMethodNotFound) {
		t.Errorf("code = %v, want %d", failure["code"], errMethodNotFound)
	}
}

func TestToolsListReturnsSevenTools(t *testing.T) {
	replies := serve(t, storeRoot(t), requestLine(1, "tools/list", ""))
	reply := decodeReply(t, replies[0])
	result, _ := reply["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 7 {
		t.Fatalf("tools/list returned %d tools, want 7", len(tools))
	}
	want := map[string]bool{
		"add": false, "search": false, "list": false, "get": false,
		"update": false, "delete": false, "reindex": false,
	}
	for _, item := range tools {
		tool, _ := item.(map[string]any)
		name, _ := tool["name"].(string)
		if _, ok := want[name]; !ok {
			t.Errorf("unexpected tool %q", name)
			continue
		}
		want[name] = true
		schema, _ := tool["inputSchema"].(map[string]any)
		if schema["type"] != "object" {
			t.Errorf("tool %q has no object schema", name)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %q is missing", name)
		}
	}
}

func TestUnknownToolIsAToolError(t *testing.T) {
	replies := serve(t, storeRoot(t), callLine(1, "frobnicate", ""))
	if text := errorText(t, replies[0]); !strings.Contains(text, "frobnicate") {
		t.Errorf("error = %q, want it to name the tool", text)
	}
}

func TestMissingArgumentIsAToolError(t *testing.T) {
	replies := serve(t, storeRoot(t), callLine(1, "add", `{"title":"No content"}`))
	if text := errorText(t, replies[0]); !strings.Contains(text, `"content"`) {
		t.Errorf("error = %q, want it to name the missing argument", text)
	}
}

func TestRoundTripThroughTheTools(t *testing.T) {
	root := storeRoot(t)
	replies := serve(t, root,
		callLine(1, "add", `{"title":"Ferret care","content":"how to look after a ferret","tags":["pets"]}`),
		callLine(2, "search", `{"query":"ferret"}`),
	)
	if len(replies) != 2 {
		t.Fatalf("session returned %d replies, want 2", len(replies))
	}

	var added map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[0])), &added); err != nil {
		t.Fatalf("add did not return JSON: %v\n%s", err, replies[0])
	}
	id, _ := added["id"].(string)
	if id == "" {
		t.Fatalf("add returned no id:\n%s", replies[0])
	}
	if added["title"] != "Ferret care" || added["content"] != "how to look after a ferret" {
		t.Errorf("add returned the wrong memory:\n%s", replies[0])
	}

	var found []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[1])), &found); err != nil {
		t.Fatalf("search did not return JSON: %v\n%s", err, replies[1])
	}
	if len(found) != 1 || found[0]["id"] != id {
		t.Errorf("search returned %v, want the added memory", found)
	}
	if _, ok := found[0]["score"]; !ok {
		t.Errorf("search result has no score:\n%s", replies[1])
	}

	replies = serve(t, root,
		callLine(3, "get", fmt.Sprintf(`{"id":%q}`, id)),
		callLine(4, "update", fmt.Sprintf(`{"id":%q,"content":"how to look after a ferret and a dog"}`, id)),
		callLine(5, "delete", fmt.Sprintf(`{"id":%q}`, id)),
		callLine(6, "get", fmt.Sprintf(`{"id":%q}`, id)),
	)
	var fetched map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[0])), &fetched); err != nil {
		t.Fatalf("get did not return JSON: %v", err)
	}
	if fetched["content"] != "how to look after a ferret" {
		t.Errorf("get returned %q, want the added body", fetched["content"])
	}
	var updated map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[1])), &updated); err != nil {
		t.Fatalf("update did not return JSON: %v", err)
	}
	if updated["content"] != "how to look after a ferret and a dog" {
		t.Errorf("update returned %q, want the replacement", updated["content"])
	}
	var deleted map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[2])), &deleted); err != nil {
		t.Fatalf("delete did not return JSON: %v", err)
	}
	if deleted["deleted"] != id {
		t.Errorf("delete returned %v, want the id confirmed", deleted)
	}
	if text := errorText(t, replies[3]); !strings.Contains(text, id) {
		t.Errorf("get after delete returned %q, want the missing memory reported", text)
	}
}

func TestSensitiveMemoryStaysOutOfSearch(t *testing.T) {
	root := storeRoot(t)
	replies := serve(t, root,
		callLine(1, "add", `{"title":"Secret","content":"the ferret hides here","sensitive":true}`),
	)
	var added map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[0])), &added); err != nil {
		t.Fatalf("add did not return JSON: %v", err)
	}
	id, _ := added["id"].(string)

	replies = serve(t, root,
		callLine(2, "search", `{"query":"ferret"}`),
		callLine(3, "list", `{}`),
		callLine(4, "list", `{"sensitive":true}`),
	)
	var found []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[0])), &found); err != nil {
		t.Fatalf("search did not return JSON: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("search returned a sensitive memory: %v", found)
	}
	var hidden []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[1])), &hidden); err != nil {
		t.Fatalf("list did not return JSON: %v", err)
	}
	if len(hidden) != 0 {
		t.Errorf("list hid nothing: %v", hidden)
	}
	var shown []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[2])), &shown); err != nil {
		t.Fatalf("list did not return JSON: %v", err)
	}
	if len(shown) != 1 || shown[0]["id"] != id {
		t.Errorf("list with sensitive returned %v, want the memory", shown)
	}
}

func TestUpdateRefusesTwoBodySources(t *testing.T) {
	root := storeRoot(t)
	replies := serve(t, root,
		callLine(1, "add", `{"title":"Kept","content":"kept body"}`),
	)
	var added map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[0])), &added); err != nil {
		t.Fatalf("add did not return JSON: %v", err)
	}
	id, _ := added["id"].(string)

	replies = serve(t, root,
		callLine(2, "update", fmt.Sprintf(`{"id":%q,"content":"replacement","append":"addition"}`, id)),
	)
	if text := errorText(t, replies[0]); !strings.Contains(text, "one of") {
		t.Errorf("error = %q, want the exclusion reported", text)
	}
}

func TestAppendJoinsWithOneNewline(t *testing.T) {
	root := storeRoot(t)
	replies := serve(t, root,
		callLine(1, "add", `{"title":"Growing","content":"first"}`),
	)
	var added map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[0])), &added); err != nil {
		t.Fatalf("add did not return JSON: %v", err)
	}
	id, _ := added["id"].(string)

	replies = serve(t, root,
		callLine(2, "update", fmt.Sprintf(`{"id":%q,"append":"second"}`, id)),
	)
	var updated map[string]any
	if err := json.Unmarshal([]byte(resultText(t, replies[0])), &updated); err != nil {
		t.Fatalf("update did not return JSON: %v", err)
	}
	if updated["content"] != "first\nsecond" {
		t.Errorf("content = %q, want one separating newline", updated["content"])
	}
}

func TestLockIsNotHeldBetweenCalls(t *testing.T) {
	root := storeRoot(t)
	// Two calls in one session both succeed, which needs the lock released
	// after the first call.
	replies := serve(t, root,
		callLine(1, "add", `{"title":"First","content":"first body"}`),
		callLine(2, "add", `{"title":"Second","content":"second body"}`),
	)
	for _, line := range replies {
		resultText(t, line)
	}

	// And the store still opens directly once the session is over.
	svc, err := service.New(root)
	if err != nil {
		t.Fatalf("opening the store after the session failed: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}
