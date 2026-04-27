package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
	coretools "bcli/internal/core/tools"
)

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type profileDTO struct {
	Kind          string                  `json:"kind"`
	Name          string                  `json:"name"`
	Profile       profile.ExternalProfile `json:"profile"`
	HasCredential bool                    `json:"hasCredential"`
}

func (s Server) tools() []map[string]any {
	return []map[string]any{
		tool("bcli.profile.list", "List MySQL and Redis profiles without exposing credentials.", objectSchema(map[string]any{}, nil)),
		tool("bcli.profile.get", "Get one profile and credential status without exposing the credential.", objectSchema(map[string]any{
			"kind": enumSchema([]string{"mysql", "redis"}, "Profile kind."),
			"name": stringSchema("Profile name."),
		}, []string{"kind", "name"})),
		tool("bcli.profile.set", "Create or replace a non-sensitive MySQL or Redis profile.", objectSchema(map[string]any{
			"kind":       enumSchema([]string{"mysql", "redis"}, "Profile kind."),
			"name":       stringSchema("Profile name."),
			"host":       stringSchema("Host name or IP address."),
			"port":       map[string]any{"type": "integer", "description": "Port number."},
			"user":       stringSchema("User name."),
			"database":   stringSchema("MySQL database name or Redis DB index."),
			"executable": stringSchema("Optional executable path."),
			"args":       arrayStringSchema("Extra client arguments."),
		}, []string{"kind", "name"})),
		tool("bcli.profile.delete", "Delete a profile and its stored credential.", objectSchema(map[string]any{
			"kind": enumSchema([]string{"mysql", "redis"}, "Profile kind."),
			"name": stringSchema("Profile name."),
		}, []string{"kind", "name"})),
		tool("bcli.auth.mysql", "Store a MySQL password in the system credential store. The password is never returned.", objectSchema(map[string]any{
			"profile":  stringSchema("Profile name. Defaults to default."),
			"password": stringSchema("MySQL password to store."),
		}, []string{"password"})),
		tool("bcli.auth.redis", "Store a Redis password in the system credential store. The password is never returned.", objectSchema(map[string]any{
			"profile":  stringSchema("Profile name. Defaults to default."),
			"password": stringSchema("Redis password to store."),
		}, []string{"password"})),
		tool("bcli.tools.uuid", "Generate a random UUID v4.", objectSchema(map[string]any{}, nil)),
		tool("bcli.tools.now", "Return the current local time in RFC3339 format.", objectSchema(map[string]any{}, nil)),
		tool("bcli.tools.base64_encode", "Base64 encode text.", objectSchema(map[string]any{
			"text": stringSchema("Text to encode."),
		}, []string{"text"})),
		tool("bcli.tools.base64_decode", "Base64 decode text.", objectSchema(map[string]any{
			"text": stringSchema("Base64 text to decode."),
		}, []string{"text"})),
		tool("bcli.tools.sha256", "Return the SHA-256 hex digest for text.", objectSchema(map[string]any{
			"text": stringSchema("Text to hash."),
		}, []string{"text"})),
		tool("bcli.tools.urlencode", "URL query-escape text.", objectSchema(map[string]any{
			"text": stringSchema("Text to encode."),
		}, []string{"text"})),
		tool("bcli.tools.urldecode", "URL query-unescape text.", objectSchema(map[string]any{
			"text": stringSchema("Text to decode."),
		}, []string{"text"})),
	}
}

func (s Server) handleToolCall(params json.RawMessage) (result any, rpcErr *rpcError) {
	defer func() {
		if value := recover(); value != nil {
			result = toolError(fmt.Sprint(value))
			rpcErr = nil
		}
	}()
	var call toolCallParams
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, invalidParams(err.Error())
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}

	var err error
	switch call.Name {
	case "bcli.profile.list":
		result, err = s.profileList()
	case "bcli.profile.get":
		result, err = s.profileGet(call.Arguments)
	case "bcli.profile.set":
		result, err = s.profileSet(call.Arguments)
	case "bcli.profile.delete":
		result, err = s.profileDelete(call.Arguments)
	case "bcli.auth.mysql":
		result, err = s.authSet("mysql", call.Arguments)
	case "bcli.auth.redis":
		result, err = s.authSet("redis", call.Arguments)
	case "bcli.tools.uuid":
		result, err = coretools.UUID()
	case "bcli.tools.now":
		result = coretools.Now()
	case "bcli.tools.base64_encode":
		result = coretools.Base64Encode(requiredString(call.Arguments, "text"))
	case "bcli.tools.base64_decode":
		result, err = coretools.Base64Decode(requiredString(call.Arguments, "text"))
	case "bcli.tools.sha256":
		result = coretools.SHA256(requiredString(call.Arguments, "text"))
	case "bcli.tools.urlencode":
		result = coretools.URLEncode(requiredString(call.Arguments, "text"))
	case "bcli.tools.urldecode":
		result, err = coretools.URLDecode(requiredString(call.Arguments, "text"))
	default:
		return nil, invalidParams("unknown tool: " + call.Name)
	}
	if err != nil {
		return toolError(err.Error()), nil
	}
	return toolTextJSON(result), nil
}

func (s Server) profileList() ([]profileDTO, error) {
	cfg, err := s.profiles.LoadConfig()
	if err != nil {
		return nil, err
	}
	return s.profileDTOs(cfg), nil
}

func (s Server) profileGet(args map[string]any) (profileDTO, error) {
	kind := requiredString(args, "kind")
	name := requiredString(args, "name")
	if !profile.IsSupportedKind(kind) {
		return profileDTO{}, fmt.Errorf("unsupported profile kind: %s", kind)
	}
	cfg, err := s.profiles.LoadConfig()
	if err != nil {
		return profileDTO{}, err
	}
	p, err := cfg.ExternalProfile(kind, name)
	if err != nil {
		return profileDTO{}, err
	}
	return s.profileDTO(kind, name, p), nil
}

func (s Server) profileSet(args map[string]any) (profileDTO, error) {
	kind := requiredString(args, "kind")
	name := requiredString(args, "name")
	if !profile.IsSupportedKind(kind) {
		return profileDTO{}, fmt.Errorf("unsupported profile kind: %s", kind)
	}
	p := profile.ExternalProfile{
		Executable: optionalString(args, "executable"),
		Args:       optionalStringSlice(args, "args"),
		Host:       optionalString(args, "host"),
		Port:       optionalInt(args, "port"),
		User:       optionalString(args, "user"),
		Database:   optionalString(args, "database"),
	}
	if err := s.profiles.Set(kind, name, p); err != nil {
		return profileDTO{}, err
	}
	return s.profileDTO(kind, name, p), nil
}

func (s Server) profileDelete(args map[string]any) (map[string]any, error) {
	kind := requiredString(args, "kind")
	name := requiredString(args, "name")
	if !profile.IsSupportedKind(kind) {
		return nil, fmt.Errorf("unsupported profile kind: %s", kind)
	}
	if err := s.profiles.Delete(kind, name); err != nil {
		return nil, err
	}
	if err := s.auth.DeleteCredential(kind, name); err != nil {
		return nil, err
	}
	return map[string]any{"kind": kind, "name": profile.NormalizeName(name), "deleted": true}, nil
}

func (s Server) authSet(kind string, args map[string]any) (map[string]any, error) {
	name := optionalString(args, "profile")
	password := requiredString(args, "password")
	if err := s.auth.StoreCredential(kind, name, password); err != nil {
		return nil, err
	}
	return map[string]any{"kind": kind, "profile": auth.NormalizeProfileName(name), "stored": true}, nil
}

func (s Server) profileDTOs(cfg profile.Config) []profileDTO {
	views := make([]profileDTO, 0, len(cfg.MySQL)+len(cfg.Redis))
	for _, kind := range []string{"mysql", "redis"} {
		profiles := cfg.MySQL
		if kind == "redis" {
			profiles = cfg.Redis
		}
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			views = append(views, s.profileDTO(kind, name, profiles[name]))
		}
	}
	return views
}

func (s Server) profileDTO(kind string, name string, p profile.ExternalProfile) profileDTO {
	hasCredential, err := s.auth.HasCredential(kind, name)
	if err != nil {
		hasCredential = false
	}
	return profileDTO{Kind: kind, Name: profile.NormalizeName(name), Profile: p, HasCredential: hasCredential}
}

func tool(name string, description string, inputSchema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": inputSchema,
	}
}

func toolTextJSON(value any) map[string]any {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return toolError(err.Error())
	}
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(data)},
		},
	}
}

func toolError(message string) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []map[string]any{
			{"type": "text", "text": message},
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func enumSchema(values []string, description string) map[string]any {
	return map[string]any{"type": "string", "enum": values, "description": description}
}

func arrayStringSchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "string"},
	}
}

func requiredString(args map[string]any, key string) string {
	value := optionalString(args, key)
	if strings.TrimSpace(value) == "" {
		panic(fmt.Sprintf("%s is required", key))
	}
	return value
}

func optionalString(args map[string]any, key string) string {
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		panic(fmt.Sprintf("%s must be a string", key))
	}
	return text
}

func optionalInt(args map[string]any, key string) int {
	value, ok := args[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		if typed < 0 || typed != float64(int(typed)) {
			panic(fmt.Sprintf("%s must be a non-negative integer", key))
		}
		return int(typed)
	case int:
		if typed < 0 {
			panic(fmt.Sprintf("%s must be a non-negative integer", key))
		}
		return typed
	default:
		panic(fmt.Sprintf("%s must be a non-negative integer", key))
	}
}

func optionalStringSlice(args map[string]any, key string) []string {
	value, ok := args[key]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		panic(fmt.Sprintf("%s must be an array of strings", key))
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			panic(fmt.Sprintf("%s must be an array of strings", key))
		}
		result = append(result, text)
	}
	return result
}
