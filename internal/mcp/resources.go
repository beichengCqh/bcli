package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"bcli/internal/core/profile"
	"bcli/internal/storage"
)

const resourceMimeJSON = "application/json"

func (s Server) handleResourcesList() (any, *rpcError) {
	resources := []map[string]any{
		resource("bcli://profiles", "profiles", "All connection profiles without secrets."),
		resource("bcli://profiles/mysql", "mysql profiles", "All MySQL profiles without secrets."),
		resource("bcli://profiles/redis", "redis profiles", "All Redis profiles without secrets."),
		resource("bcli://config/paths", "config paths", "bcli config read/write paths."),
	}
	cfg, err := s.profiles.LoadConfig()
	if err != nil {
		return nil, internalError(err.Error())
	}
	for _, view := range s.profileDTOs(cfg) {
		resources = append(resources, resource(
			fmt.Sprintf("bcli://profiles/%s/%s", view.Kind, escapeResourceSegment(view.Name)),
			fmt.Sprintf("%s/%s", view.Kind, view.Name),
			"One connection profile without secrets.",
		))
	}
	return map[string]any{"resources": resources}, nil
}

func (s Server) resourceTemplates() []map[string]any {
	return []map[string]any{
		{
			"uriTemplate": "bcli://profiles/{kind}/{name}",
			"name":        "profile by kind and name",
			"description": "Read one MySQL or Redis profile without secrets.",
			"mimeType":    resourceMimeJSON,
		},
	}
}

func (s Server) handleResourceRead(params json.RawMessage) (any, *rpcError) {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, invalidParams(err.Error())
	}
	value, err := s.readResourceValue(req.URI)
	if err != nil {
		return nil, invalidParams(err.Error())
	}
	text, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, internalError(err.Error())
	}
	return map[string]any{
		"contents": []map[string]any{
			{
				"uri":      req.URI,
				"mimeType": resourceMimeJSON,
				"text":     string(text),
			},
		},
	}, nil
}

func (s Server) readResourceValue(uri string) (any, error) {
	switch uri {
	case "bcli://profiles":
		cfg, err := s.profiles.LoadConfig()
		if err != nil {
			return nil, err
		}
		return s.profileDTOs(cfg), nil
	case "bcli://profiles/mysql":
		return s.profileDTOsByKind("mysql")
	case "bcli://profiles/redis":
		return s.profileDTOsByKind("redis")
	case "bcli://config/paths":
		readPath, readErr := storage.ConfigReadPath()
		writePath, writeErr := storage.ConfigWritePath()
		return map[string]any{
			"readPath":    readPath,
			"readError":   errorString(readErr),
			"writePath":   writePath,
			"writeError":  errorString(writeErr),
			"configEnv":   "BCLI_CONFIG",
			"defaultRoot": "~/.bcli",
		}, nil
	default:
		parts := strings.Split(strings.TrimPrefix(uri, "bcli://profiles/"), "/")
		if len(parts) == 2 && profile.IsSupportedKind(parts[0]) {
			name, err := unescapeResourceSegment(parts[1])
			if err != nil {
				return nil, err
			}
			cfg, err := s.profiles.LoadConfig()
			if err != nil {
				return nil, err
			}
			p, err := cfg.ExternalProfile(parts[0], name)
			if err != nil {
				return nil, err
			}
			return s.profileDTO(parts[0], name, p), nil
		}
		return nil, fmt.Errorf("unknown resource uri: %s", uri)
	}
}

func escapeResourceSegment(value string) string {
	return url.PathEscape(value)
}

func unescapeResourceSegment(value string) (string, error) {
	unescaped, err := url.PathUnescape(value)
	if err != nil {
		return "", fmt.Errorf("invalid resource uri escaping: %w", err)
	}
	return unescaped, nil
}

func (s Server) profileDTOsByKind(kind string) ([]profileDTO, error) {
	cfg, err := s.profiles.LoadConfig()
	if err != nil {
		return nil, err
	}
	all := s.profileDTOs(cfg)
	filtered := make([]profileDTO, 0, len(all))
	for _, item := range all {
		if item.Kind == kind {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func resource(uri string, name string, description string) map[string]any {
	return map[string]any{
		"uri":         uri,
		"name":        name,
		"description": description,
		"mimeType":    resourceMimeJSON,
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
