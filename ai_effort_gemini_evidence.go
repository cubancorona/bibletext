package bibletext

import (
	"encoding/json"
	"strings"
	"time"
)

// generateContent's modelVersion identifies what actually answered an alias.
// A rolling alias can move again; cache only the model ID and observation time.
// https://aistudio.google.com/api-reference/generate-content#Resource:GenerateContentResponse
type geminiAliasObservation struct {
	Target  string `json:"target"`
	Checked int64  `json:"checked"`
}

func geminiResponseTargetKey(alias string) string {
	return "ai.geminiResponseTarget." + alias
}

func validGeminiAlias(alias string) bool {
	return len(alias) <= 128 && strings.HasPrefix(alias, "gemini-") && strings.HasSuffix(alias, "-latest") && publicModelSlug.MatchString(alias)
}

func rememberGeminiResponseTarget(prefs prefStore, alias, canonical string) {
	if prefs == nil || !validGeminiAlias(alias) || len(canonical) > 128 {
		return
	}
	target := (geminiCatalogModel{Name: alias, Version: canonical}).documentModel()
	if target != "" {
		data, _ := json.Marshal(geminiAliasObservation{Target: target, Checked: time.Now().Unix()})
		prefs.SetString(geminiResponseTargetKey(alias), string(data))
	}
}

func recentGeminiResponseTarget(prefs prefStore, alias string) string {
	if prefs == nil || !validGeminiAlias(alias) {
		return ""
	}
	var obs geminiAliasObservation
	if json.Unmarshal([]byte(prefs.String(geminiResponseTargetKey(alias))), &obs) != nil {
		return ""
	}
	age := time.Since(time.Unix(obs.Checked, 0))
	if age < 0 || age >= 24*time.Hour || len(obs.Target) > 128 {
		return ""
	}
	return (geminiCatalogModel{Name: alias, Version: obs.Target}).documentModel()
}
