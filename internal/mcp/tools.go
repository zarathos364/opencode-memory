package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zarathos364/opencode-memory/internal/indexer"
	"github.com/zarathos364/opencode-memory/internal/service"
)

type definition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func schema(required []string, properties map[string]any) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func toolDefinitions() []definition {
	text := map[string]any{"type": "string", "description": "Memory body text."}
	return []definition{
		{
			Name:        "add",
			Description: "Store a new memory and return it.",
			InputSchema: schema([]string{"title", "content"}, map[string]any{
				"title":     map[string]any{"type": "string", "description": "Short, descriptive title."},
				"content":   text,
				"tags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"sensitive": map[string]any{"type": "boolean", "description": "Store without making it searchable."},
			}),
		},
		{
			Name:        "search",
			Description: "Find memories by their text, best matches first. A result marked weak matched only part of the query, and an empty list means nothing matched. A result carries the title and tags, not the body.",
			InputSchema: schema([]string{"query"}, map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query."},
				"limit": map[string]any{"type": "integer", "description": "Maximum number of results; zero or less uses the default."},
			}),
		},
		{
			Name:        "list",
			Description: "List stored memories, oldest first. An entry carries the title and tags, not the body.",
			InputSchema: schema([]string{}, map[string]any{
				"sensitive": map[string]any{"type": "boolean", "description": "Include memories kept out of search."},
			}),
		},
		{
			Name:        "get",
			Description: "Print one memory with its header fields and body.",
			InputSchema: schema([]string{"id"}, map[string]any{
				"id": map[string]any{"type": "string", "description": "Memory identifier."},
			}),
		},
		{
			Name:        "update",
			Description: "Change part of a memory while keeping its id and creation time. One body source per call.",
			InputSchema: schema([]string{"id"}, map[string]any{
				"id":        map[string]any{"type": "string", "description": "Memory identifier."},
				"content":   map[string]any{"type": "string", "description": "Replace the body."},
				"append":    map[string]any{"type": "string", "description": "Add text to the end of the body."},
				"replace":   map[string]any{"type": "string", "description": "Text inside the body to replace; use with."},
				"with":      map[string]any{"type": "string", "description": "Replacement text for replace."},
				"title":     map[string]any{"type": "string", "description": "Replace the title."},
				"tags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Replace the tags."},
				"sensitive": map[string]any{"type": "boolean", "description": "Replace the sensitive flag."},
			}),
		},
		{
			Name:        "delete",
			Description: "Remove one memory and its index entry.",
			InputSchema: schema([]string{"id"}, map[string]any{
				"id": map[string]any{"type": "string", "description": "Memory identifier."},
			}),
		},
		{
			Name:        "reindex",
			Description: "Rebuild the index from the stored files.",
			InputSchema: schema([]string{}, map[string]any{}),
		},
	}
}

func (s *Server) call(id json.RawMessage, raw json.RawMessage) []byte {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || params.Name == "" {
		return errorReply(id, errInvalidParams, "tools/call needs a tool name")
	}
	payload, err := s.dispatch(params.Name, params.Arguments)
	if err != nil {
		return resultReply(id, map[string]any{
			"content": []any{map[string]any{"type": "text", "text": err.Error()}},
			"isError": true,
		})
	}
	text, err := json.Marshal(payload)
	if err != nil {
		return errorReply(id, errParse, "encode result")
	}
	return resultReply(id, map[string]any{
		"content": []any{map[string]any{"type": "text", "text": string(text)}},
	})
}

// open holds the store for one tool call and releases it after, so the CLI
// keeps working while the server runs.
func (s *Server) open() (*service.MemoryService, error) {
	svc, err := service.New(s.Root)
	if err != nil {
		if indexer.IsLockError(err) {
			return nil, fmt.Errorf("%v: another instance is using this store", err)
		}
		return nil, err
	}
	return svc, nil
}

// withStore opens the store, hands it to work and closes it afterwards. The
// release reports nothing: a tool result describes the tool call, not the
// release after it.
func (s *Server) withStore(work func(*service.MemoryService) (any, error)) (any, error) {
	svc, err := s.open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = svc.Close() }()
	return work(svc)
}

func (s *Server) dispatch(name string, args map[string]any) (any, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "add":
		return s.toolAdd(args)
	case "search":
		return s.toolSearch(args)
	case "list":
		return s.toolList(args)
	case "get":
		return s.toolGet(args)
	case "update":
		return s.toolUpdate(args)
	case "delete":
		return s.toolDelete(args)
	case "reindex":
		return s.toolReindex(args)
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

func (s *Server) toolAdd(args map[string]any) (any, error) {
	title, err := reqString("add", args, "title")
	if err != nil {
		return nil, err
	}
	content, err := reqString("add", args, "content")
	if err != nil {
		return nil, err
	}
	tags, _, err := optTags(args)
	if err != nil {
		return nil, err
	}
	sensitive, _, err := optBool(args, "sensitive")
	if err != nil {
		return nil, err
	}

	return s.withStore(func(svc *service.MemoryService) (any, error) {
		created, err := svc.Create(title, content, tags, sensitive)
		if err != nil {
			return nil, err
		}
		return service.NewMemoryJSON(created, 0), nil
	})
}

func (s *Server) toolSearch(args map[string]any) (any, error) {
	query, err := reqString("search", args, "query")
	if err != nil {
		return nil, err
	}
	limit, err := optLimit(args)
	if err != nil {
		return nil, err
	}

	return s.withStore(func(svc *service.MemoryService) (any, error) {
		results, err := svc.Search(query, limit)
		if err != nil {
			return nil, err
		}
		return service.NewMemorySummaryResults(results), nil
	})
}

func (s *Server) toolList(args map[string]any) (any, error) {
	showSensitive, _, err := optBool(args, "sensitive")
	if err != nil {
		return nil, err
	}

	return s.withStore(func(svc *service.MemoryService) (any, error) {
		memories, err := svc.List()
		if err != nil {
			return nil, err
		}
		return service.NewMemorySummaryList(service.VisibleMemories(memories, showSensitive)), nil
	})
}

func (s *Server) toolGet(args map[string]any) (any, error) {
	id, err := reqString("get", args, "id")
	if err != nil {
		return nil, err
	}

	return s.withStore(func(svc *service.MemoryService) (any, error) {
		m, err := svc.Get(id)
		if err != nil {
			return nil, err
		}
		return service.NewMemoryJSON(m, 0), nil
	})
}

func (s *Server) toolUpdate(args map[string]any) (any, error) {
	id, err := reqString("update", args, "id")
	if err != nil {
		return nil, err
	}
	content, hasContent, err := optString(args, "content")
	if err != nil {
		return nil, err
	}
	addition, hasAppend, err := optString(args, "append")
	if err != nil {
		return nil, err
	}
	find, hasFind, err := optString(args, "replace")
	if err != nil {
		return nil, err
	}
	substitute, hasSubstitute, err := optString(args, "with")
	if err != nil {
		return nil, err
	}
	if hasFind != hasSubstitute {
		return nil, fmt.Errorf("update needs replace and with together")
	}
	// One body source per call, like the CLI.
	sources := 0
	for _, has := range []bool{hasContent, hasAppend, hasFind} {
		if has {
			sources++
		}
	}
	if sources > 1 {
		return nil, fmt.Errorf("update takes one of content, append or replace, not several")
	}
	if hasAppend && strings.TrimSpace(addition) == "" {
		return nil, fmt.Errorf("update append holds no text")
	}

	title, hasTitle, err := optString(args, "title")
	if err != nil {
		return nil, err
	}
	tags, hasTags, err := optTags(args)
	if err != nil {
		return nil, err
	}
	sensitive, hasSensitive, err := optBool(args, "sensitive")
	if err != nil {
		return nil, err
	}
	if sources == 0 && !hasTitle && !hasTags && !hasSensitive {
		return nil, fmt.Errorf("update needs a field to change")
	}

	patch := service.Patch{}
	if hasTitle {
		patch.Title = &title
	}
	if hasTags {
		patch.Tags = &tags
	}
	if hasSensitive {
		patch.Sensitive = &sensitive
	}
	if hasContent {
		patch.Content = &content
	}
	if hasFind {
		patch.Replace = &service.Replacement{Find: find, With: substitute}
	}

	return s.withStore(func(svc *service.MemoryService) (any, error) {
		if hasAppend {
			current, err := svc.Get(id)
			if err != nil {
				return nil, err
			}
			joined := service.AppendBody(current.Content, addition)
			patch.Content = &joined
		}
		updated, err := svc.Update(id, patch)
		if err != nil {
			return nil, err
		}
		return service.NewMemoryJSON(updated, 0), nil
	})
}

func (s *Server) toolDelete(args map[string]any) (any, error) {
	id, err := reqString("delete", args, "id")
	if err != nil {
		return nil, err
	}

	return s.withStore(func(svc *service.MemoryService) (any, error) {
		if err := svc.Delete(id); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": id}, nil
	})
}

func (s *Server) toolReindex(_ map[string]any) (any, error) {
	return s.withStore(func(svc *service.MemoryService) (any, error) {
		stored, err := svc.Reindex()
		if err != nil {
			return nil, err
		}
		return map[string]any{"reindexed": stored}, nil
	})
}

func reqString(tool string, args map[string]any, name string) (string, error) {
	value, ok := args[name]
	text, isString := value.(string)
	if !ok || !isString {
		return "", fmt.Errorf("%s needs %q", tool, name)
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("%s needs text in %q", tool, name)
	}
	return text, nil
}

func optString(args map[string]any, name string) (string, bool, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return "", false, nil
	}
	text, isString := value.(string)
	if !isString {
		return "", false, fmt.Errorf("%q must be a string", name)
	}
	return text, true, nil
}

func optBool(args map[string]any, name string) (bool, bool, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return false, false, nil
	}
	setting, isBool := value.(bool)
	if !isBool {
		return false, false, fmt.Errorf("%q must be a boolean", name)
	}
	return setting, true, nil
}

func optTags(args map[string]any) ([]string, bool, error) {
	value, ok := args["tags"]
	if !ok || value == nil {
		return nil, false, nil
	}
	raw, isSlice := value.([]any)
	if !isSlice {
		return nil, false, fmt.Errorf("%q must be an array of strings", "tags")
	}
	tags := make([]string, 0, len(raw))
	for _, item := range raw {
		tag, isString := item.(string)
		if !isString {
			return nil, false, fmt.Errorf("%q must be an array of strings", "tags")
		}
		tags = append(tags, tag)
	}
	return tags, true, nil
}

func optLimit(args map[string]any) (int, error) {
	value, ok := args["limit"]
	if !ok || value == nil {
		return indexer.DefaultLimit, nil
	}
	number, isNumber := value.(float64)
	if !isNumber {
		return 0, fmt.Errorf("%q must be a number", "limit")
	}
	return int(number), nil
}
