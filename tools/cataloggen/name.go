package main

import (
	"strings"
)

// verbWords maps the verb-shaped tokens the operationIds use to the canonical
// verb that ends a tool name. A token absent here is part of the entity, so an
// identifier that names no verb keeps its snake_case form.
var verbWords = map[string]string{
	"list":         "list",
	"getall":       "list",
	"get":          "get", // resolved by the method and the path
	"inspect":      "inspect",
	"describe":     "inspect",
	"show":         "show",
	"create":       "create",
	"delete":       "delete",
	"remove":       "delete",
	"update":       "update",
	"patch":        "patch",
	"start":        "start",
	"stop":         "stop",
	"restart":      "restart",
	"kill":         "kill",
	"pause":        "pause",
	"unpause":      "unpause",
	"attach":       "attach",
	"exec":         "exec",
	"logs":         "logs",
	"stats":        "stats",
	"top":          "top",
	"scale":        "scale",
	"resize":       "resize",
	"rollback":     "rollback",
	"drain":        "drain",
	"cordon":       "cordon",
	"migrate":      "migrate",
	"associate":    "associate",
	"dissociate":   "dissociate",
	"backup":       "backup",
	"restore":      "restore",
	"install":      "install",
	"uninstall":    "delete",
	"upgrade":      "upgrade",
	"search":       "search",
	"test":         "test",
	"ping":         "ping",
	"configure":    "configure",
	"validate":     "validate",
	"authenticate": "authenticate",
	"logout":       "logout",
	"upload":       "upload",
	"invoke":       "invoke",
	"toggle":       "toggle",
	"prune":        "prune",
	"init":         "init",
	"check":        "check",
	"count":        "count",
	"counts":       "count",
	"summary":      "summary",
	"version":      "version",
	"info":         "info",
	"status":       "status",
	"events":       "events",
	"snapshot":     "snapshot",
	"snapshots":    "snapshot",
	"clear":        "clear",
	"collect":      "collect",
}

// entityStopWords add nothing to a name: the HTTP verb already carries "all"
// and "for".
var entityStopWords = map[string]bool{
	"all": true, "for": true, "by": true, "of": true, "the": true,
	"and": true, "in": true, "with": true, "on": true, "to": true,
}

// toolName returns the curated name when there is one, otherwise the derived
// <entity>_<verb> form.
func toolName(m mapping, op *operation) (string, error) {
	if n, ok := m.Rewrites[op.source+"/"+op.id]; ok {
		return n, nil
	}
	if n, ok := m.Rewrites[op.id]; ok {
		return n, nil
	}
	words := applyAliases(snakeWords(op.id), m.Aliases)
	if len(words) == 1 {
		return words[0], nil
	}
	if v, ok := resolveVerb(words[len(words)-1], op.path); ok {
		entity := words[:len(words)-1]
		// "GetKubernetesPodLogs" keeps only the entity: the leading verb is the
		// same in every name and carries nothing.
		if _, isVerb := verbWords[entity[0]]; isVerb && len(entity) > 1 {
			entity = entity[1:]
		}
		if name := join(entity, v); name != "" {
			return name, nil
		}
	}
	if v, ok := resolveVerb(words[0], op.path); ok {
		if name := join(words[1:], v); name != "" {
			return name, nil
		}
	}
	return strings.Join(words, "_"), nil
}

// applyAliases rejoins the words a camelCase split broke apart: "GitOps" comes
// out as "git" and "ops" and becomes "gitops" again.
func applyAliases(words []string, aliases map[string]string) []string {
	if len(aliases) == 0 {
		return words
	}
	out := make([]string, 0, len(words))
	for i := 0; i < len(words); i++ {
		merged := false
		for n := len(words) - i; n >= 2; n-- {
			if alias, ok := aliases[strings.Join(words[i:i+n], "_")]; ok {
				out = append(out, alias)
				i += n - 1
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, words[i])
		}
	}
	return out
}

// resolveVerb turns a verb-shaped token into the verb ending the tool name. A
// bare "get" or "getAll" lists a collection and inspects a single item, which
// the path tells apart: an item path ends with its parameter.
func resolveVerb(word, path string) (string, bool) {
	v, ok := verbWords[word]
	if !ok {
		return "", false
	}
	if v == "get" || v == "list" && word == "getall" {
		if strings.HasSuffix(path, "}") {
			return "inspect", true
		}
		return "list", true
	}
	return v, true
}

// join drops the stop words that make a derived name long, then appends the
// verb.
func join(entity []string, verb string) string {
	kept := make([]string, 0, len(entity))
	for _, w := range entity {
		if w == "" || entityStopWords[w] {
			continue
		}
		kept = append(kept, w)
	}
	kept = append(kept, verb)
	return strings.Join(kept, "_")
}

// snakeWords splits a camelCase or PascalCase identifier into lowercase words.
// An acronym stays one word: "GitOps" and "LDAPCheck" do not become "git_ops".
func snakeWords(s string) []string {
	var words []string
	var cur []rune
	runes := []rune(s)
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '_' || r == '-' || r == '.' || r == '/' || r == ' ' || r == '{' || r == '}':
			flush()
		case isUpper(r):
			prev := rune(0)
			if len(cur) > 0 {
				prev = cur[len(cur)-1]
			}
			next := rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			// "CustomTemplate": split before the upper letter that starts a word.
			// "LDAPCheck": keep the acronym, split before "C".
			if len(cur) > 0 && (!isUpper(prev) || (isUpper(prev) && next != 0 && !isUpper(next))) {
				flush()
			}
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return words
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
