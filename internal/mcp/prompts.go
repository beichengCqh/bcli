package mcp

import (
	"encoding/json"
	"fmt"
)

func (s Server) prompts() []map[string]any {
	return []map[string]any{
		prompt("bcli.prompt.create_mysql_profile", "Create MySQL profile", "Guide an AI through creating a non-sensitive MySQL profile, then storing its password separately.", []map[string]any{
			promptArgument("name", "Profile name.", true),
			promptArgument("host", "MySQL host.", false),
			promptArgument("port", "MySQL port.", false),
			promptArgument("user", "MySQL user.", false),
			promptArgument("database", "MySQL database.", false),
		}),
		prompt("bcli.prompt.create_redis_profile", "Create Redis profile", "Guide an AI through creating a non-sensitive Redis profile, then storing its password separately.", []map[string]any{
			promptArgument("name", "Profile name.", true),
			promptArgument("host", "Redis host.", false),
			promptArgument("port", "Redis port.", false),
			promptArgument("user", "Redis user.", false),
			promptArgument("database", "Redis DB index.", false),
		}),
		prompt("bcli.prompt.inspect_profiles", "Inspect profiles", "Inspect configured profiles without exposing credentials.", nil),
		prompt("bcli.prompt.rotate_profile_password", "Rotate profile password", "Guide password rotation using auth tools without returning the password.", []map[string]any{
			promptArgument("kind", "Profile kind: mysql or redis.", true),
			promptArgument("name", "Profile name.", true),
		}),
	}
}

func (s Server) handlePromptGet(params json.RawMessage) (any, *rpcError) {
	var req struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, invalidParams(err.Error())
	}

	text, err := promptText(req.Name, req.Arguments)
	if err != nil {
		return nil, invalidParams(err.Error())
	}
	return map[string]any{
		"description": text,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": map[string]any{
					"type": "text",
					"text": text,
				},
			},
		},
	}, nil
}

func promptText(name string, args map[string]string) (string, error) {
	switch name {
	case "bcli.prompt.create_mysql_profile":
		return fmt.Sprintf("Create or update a MySQL profile named %q using bcli.profile.set. Store only non-sensitive fields in the profile. If a password is provided by the user, store it with bcli.auth.mysql and never echo it back.", args["name"]), nil
	case "bcli.prompt.create_redis_profile":
		return fmt.Sprintf("Create or update a Redis profile named %q using bcli.profile.set. Store only non-sensitive fields in the profile. If a password is provided by the user, store it with bcli.auth.redis and never echo it back.", args["name"]), nil
	case "bcli.prompt.inspect_profiles":
		return "Use bcli.profile.list or bcli://profiles to inspect profiles. Report host, port, user, database, executable, args, and hasCredential only. Never ask for or reveal stored credentials.", nil
	case "bcli.prompt.rotate_profile_password":
		return fmt.Sprintf("Rotate the password for %s profile %q. Ask the user for the new password if it was not provided, call the matching bcli.auth.* tool, then confirm only that the credential was stored.", args["kind"], args["name"]), nil
	default:
		return "", fmt.Errorf("unknown prompt: %s", name)
	}
}

func prompt(name string, title string, description string, arguments []map[string]any) map[string]any {
	value := map[string]any{
		"name":        name,
		"title":       title,
		"description": description,
	}
	if len(arguments) > 0 {
		value["arguments"] = arguments
	}
	return value
}

func promptArgument(name string, description string, required bool) map[string]any {
	return map[string]any{
		"name":        name,
		"description": description,
		"required":    required,
	}
}
