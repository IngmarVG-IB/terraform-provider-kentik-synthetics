package main

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// kentikAPIManifest mirrors the kentik-api.json format read by the kentikcommunity indexer.
type kentikAPIManifest struct {
	SchemaSource struct {
		Repo string `json:"repo"`
		Ref  string `json:"ref"`
	} `json:"schema_source"`
	Services []struct {
		Service   string   `json:"service"`
		Version   string   `json:"version"`
		Stability string   `json:"stability"`
		Status    string   `json:"status"`
		Surface   []string `json:"surface"`
	} `json:"services"`
	Legacy []struct {
		API    string `json:"api"`
		Status string `json:"status"`
	} `json:"legacy"`
}

func TestKentikAPIManifest(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("kentik-api.json")
	if err != nil {
		t.Fatalf("read kentik-api.json: %v", err)
	}

	var m kentikAPIManifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode kentik-api.json: %v", err)
	}

	if m.SchemaSource.Repo == "" || m.SchemaSource.Ref == "" {
		t.Errorf("schema_source.repo and schema_source.ref are required")
	}
	if len(m.Services) == 0 {
		t.Errorf("at least one service is required")
	}

	version := regexp.MustCompile(`^v\d{6}(alpha\d+|beta\d+)?$`)
	stability := map[string]bool{"stable": true, "beta": true, "alpha": true}
	status := map[string]bool{"supported": true, "partial": true, "experimental": true, "deprecated": true}
	for i, s := range m.Services {
		if s.Service == "" {
			t.Errorf("services[%d].service is empty", i)
		}
		if !version.MatchString(s.Version) {
			t.Errorf("services[%d].version %q is not a Kentik API version", i, s.Version)
		}
		if !stability[s.Stability] {
			t.Errorf("services[%d].stability %q is invalid", i, s.Stability)
		}
		if !status[s.Status] {
			t.Errorf("services[%d].status %q is invalid", i, s.Status)
		}
		if len(s.Surface) == 0 {
			t.Errorf("services[%d].surface is empty", i)
		}
	}
	// The schema ref must be the api-schema-public version pinned in go.mod.
	if m.SchemaSource.Repo == "github.com/kentik/api-schema-public" {
		gomod, err := os.ReadFile("go.mod")
		if err != nil {
			t.Fatalf("read go.mod: %v", err)
		}
		pin := regexp.MustCompile(`(?m)^\s*github\.com/kentik/api-schema-public\s+(\S+)`).FindSubmatch(gomod)
		if pin == nil || string(pin[1]) != m.SchemaSource.Ref {
			t.Errorf("schema_source.ref %q does not match go.mod (%q)", m.SchemaSource.Ref, pin)
		}
	}
	for i, l := range m.Legacy {
		if l.API == "" || !status[l.Status] {
			t.Errorf("legacy[%d] is invalid: %+v", i, l)
		}
	}
}
