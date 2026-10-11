package bibletext

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"
)

// Optional AI requests carry content authenticated to a provider account.
// Keep release declarations aligned with that data flow and the supported
// providers' purposes. This guard checks plist types and dictionary ownership,
// which matching strings cannot establish.
func TestPrivacyManifestDeclaresProviderDataAndRequiredReasons(t *testing.T) {
	raw, err := os.ReadFile("cmd/mobile/PrivacyInfo.xcprivacy")
	if err != nil {
		t.Fatal(err)
	}
	var plist privacyManifestNode
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&plist); err != nil {
		t.Fatalf("privacy manifest is not XML: %v", err)
	}
	var trailing privacyManifestNode
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("privacy manifest has trailing XML: %v", err)
	}
	if plist.XMLName != (xml.Name{Local: "plist"}) ||
		len(plist.Attributes) != 1 || plist.Attributes[0].Name != (xml.Name{Local: "version"}) ||
		plist.Attributes[0].Value != "1.0" || strings.TrimSpace(plist.Text) != "" ||
		len(plist.Children) != 1 {
		t.Fatal("privacy manifest must have one dictionary inside a version 1.0 plist")
	}
	root := privacyManifestDictionary(t, plist.Children[0],
		"NSPrivacyTracking", "NSPrivacyTrackingDomains", "NSPrivacyCollectedDataTypes", "NSPrivacyAccessedAPITypes")
	privacyManifestBoolean(t, root["NSPrivacyTracking"], false)
	if domains := privacyManifestArray(t, root["NSPrivacyTrackingDomains"]); len(domains) != 0 {
		t.Fatal("privacy manifest must not declare tracking domains")
	}

	wantData := map[string][]string{
		"NSPrivacyCollectedDataTypeOtherUserContent": {
			"NSPrivacyCollectedDataTypePurposeAppFunctionality", "NSPrivacyCollectedDataTypePurposeOther",
		},
		"NSPrivacyCollectedDataTypeSearchHistory": {
			"NSPrivacyCollectedDataTypePurposeAppFunctionality", "NSPrivacyCollectedDataTypePurposeOther",
		},
		"NSPrivacyCollectedDataTypeUserID": {"NSPrivacyCollectedDataTypePurposeAppFunctionality"},
	}
	collected := privacyManifestArray(t, root["NSPrivacyCollectedDataTypes"])
	if len(collected) != len(wantData) {
		t.Fatalf("privacy manifest declares %d collected-data types, want %d", len(collected), len(wantData))
	}
	seenData := make(map[string]bool)
	for _, entry := range collected {
		dict := privacyManifestDictionary(t, entry, "NSPrivacyCollectedDataType",
			"NSPrivacyCollectedDataTypeLinked", "NSPrivacyCollectedDataTypeTracking", "NSPrivacyCollectedDataTypePurposes")
		kind := privacyManifestString(t, dict["NSPrivacyCollectedDataType"])
		purposes, ok := wantData[kind]
		if !ok || seenData[kind] {
			t.Fatalf("unexpected or duplicate collected-data type %q", kind)
		}
		seenData[kind] = true
		privacyManifestBoolean(t, dict["NSPrivacyCollectedDataTypeLinked"], true)
		privacyManifestBoolean(t, dict["NSPrivacyCollectedDataTypeTracking"], false)
		privacyManifestStringSet(t, dict["NSPrivacyCollectedDataTypePurposes"], purposes)
	}

	// Keep the required-reason API declarations independent of the collected
	// data changes: packaging still depends on all three existing reasons.
	wantAPIs := map[string]string{
		"NSPrivacyAccessedAPICategoryFileTimestamp":  "C617.1",
		"NSPrivacyAccessedAPICategorySystemBootTime": "35F9.1",
		"NSPrivacyAccessedAPICategoryUserDefaults":   "CA92.1",
	}
	accessed := privacyManifestArray(t, root["NSPrivacyAccessedAPITypes"])
	if len(accessed) != len(wantAPIs) {
		t.Fatalf("privacy manifest declares %d required-reason APIs, want %d", len(accessed), len(wantAPIs))
	}
	seenAPIs := make(map[string]bool)
	for _, entry := range accessed {
		dict := privacyManifestDictionary(t, entry, "NSPrivacyAccessedAPIType", "NSPrivacyAccessedAPITypeReasons")
		kind := privacyManifestString(t, dict["NSPrivacyAccessedAPIType"])
		reason, ok := wantAPIs[kind]
		if !ok || seenAPIs[kind] {
			t.Fatalf("unexpected or duplicate required-reason API %q", kind)
		}
		seenAPIs[kind] = true
		privacyManifestStringSet(t, dict["NSPrivacyAccessedAPITypeReasons"], []string{reason})
	}
}

// Only the XML plist types used by this manifest are needed. No Apple command
// or host-specific parser is required by the release guard.
type privacyManifestNode struct {
	XMLName    xml.Name
	Attributes []xml.Attr            `xml:",any,attr"`
	Text       string                `xml:",chardata"`
	Children   []privacyManifestNode `xml:",any"`
}

func privacyManifestContainer(t *testing.T, node privacyManifestNode, tag string) {
	t.Helper()
	if node.XMLName != (xml.Name{Local: tag}) || len(node.Attributes) != 0 || strings.TrimSpace(node.Text) != "" {
		t.Fatalf("expected a plist %s, got <%s> with attributes %v and text %q", tag, node.XMLName.Local, node.Attributes, node.Text)
	}
}

func privacyManifestDictionary(t *testing.T, node privacyManifestNode, keys ...string) map[string]privacyManifestNode {
	t.Helper()
	privacyManifestContainer(t, node, "dict")
	if len(node.Children) != 2*len(keys) {
		t.Fatalf("plist dictionary has %d elements, want %d key/value pairs", len(node.Children), len(keys))
	}
	allowed := make(map[string]bool, len(keys))
	for _, key := range keys {
		allowed[key] = true
	}
	values := make(map[string]privacyManifestNode, len(keys))
	for i := 0; i < len(node.Children); i += 2 {
		keyNode := node.Children[i]
		if keyNode.XMLName != (xml.Name{Local: "key"}) || len(keyNode.Attributes) != 0 || len(keyNode.Children) != 0 {
			t.Fatalf("plist dictionary element %d is not a scalar key", i)
		}
		key := keyNode.Text
		_, duplicate := values[key]
		if !allowed[key] || duplicate {
			t.Fatalf("unexpected or duplicate plist dictionary key %q", key)
		}
		values[key] = node.Children[i+1]
	}
	return values
}

func privacyManifestArray(t *testing.T, node privacyManifestNode) []privacyManifestNode {
	t.Helper()
	privacyManifestContainer(t, node, "array")
	return node.Children
}

func privacyManifestString(t *testing.T, node privacyManifestNode) string {
	t.Helper()
	if node.XMLName != (xml.Name{Local: "string"}) || len(node.Attributes) != 0 || len(node.Children) != 0 ||
		node.Text == "" || strings.TrimSpace(node.Text) != node.Text {
		t.Fatalf("expected a nonempty plist string, got <%s> containing %q", node.XMLName.Local, node.Text)
	}
	return node.Text
}

func privacyManifestBoolean(t *testing.T, node privacyManifestNode, want bool) {
	t.Helper()
	tag := "false"
	if want {
		tag = "true"
	}
	privacyManifestContainer(t, node, tag)
	if len(node.Children) != 0 {
		t.Fatalf("plist boolean <%s> must not contain other elements", tag)
	}
}

func privacyManifestStringSet(t *testing.T, node privacyManifestNode, want []string) {
	t.Helper()
	values := privacyManifestArray(t, node)
	if len(values) != len(want) {
		t.Fatalf("plist string array has %d entries, want %v", len(values), want)
	}
	remaining := make(map[string]bool, len(want))
	for _, value := range want {
		remaining[value] = true
	}
	for _, entry := range values {
		value := privacyManifestString(t, entry)
		if !remaining[value] {
			t.Fatalf("unexpected or duplicate plist string %q; want %v", value, want)
		}
		delete(remaining, value)
	}
}
