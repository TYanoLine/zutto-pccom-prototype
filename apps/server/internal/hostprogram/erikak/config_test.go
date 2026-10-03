package erikak

import "testing"

func TestDefaultConfigEnablesAllFeaturesAndProtocols(t *testing.T) {
	cfg := DefaultConfig()
	for feature := range knownFeatures {
		if !cfg.FeatureEnabled(feature) {
			t.Fatalf("feature %q should default to enabled", feature)
		}
	}
	for protocol := range knownProtocols {
		if !cfg.ProtocolEnabled(protocol) {
			t.Fatalf("protocol %q should default to enabled", protocol)
		}
	}
	if got := len(cfg.EnabledTransferProtocols()); got != len(transferProtocolCatalog) {
		t.Fatalf("expected %d enabled protocols, got %d", len(transferProtocolCatalog), got)
	}
}

func TestParseConfigJSONOverlaysDefaults(t *testing.T) {
	cfg, err := ParseConfigJSON([]byte(`{
		"features": {
			"file": false,
			"junk": false
		},
		"transfer_protocols": {
			"nmodem": false,
			"ymodem_g": false
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FeatureEnabled(FeatureFile) || cfg.FeatureEnabled(FeatureJunk) {
		t.Fatal("explicitly disabled features remained enabled")
	}
	if !cfg.FeatureEnabled(FeatureBoard) {
		t.Fatal("omitted feature should retain default enabled state")
	}
	if cfg.ProtocolEnabled("nmodem") || cfg.ProtocolEnabled("ymodem_g") {
		t.Fatal("explicitly disabled protocols remained enabled")
	}
	if !cfg.ProtocolEnabled("zmodem") {
		t.Fatal("omitted protocol should retain default enabled state")
	}
}

func TestParseConfigJSONRejectsUnknownKeys(t *testing.T) {
	if _, err := ParseConfigJSON([]byte(`{"features":{"fiel":false}}`)); err == nil {
		t.Fatal("misspelled feature key should be rejected")
	}
	if _, err := ParseConfigJSON([]byte(`{"transfer_protocols":{"nmodem2":false}}`)); err == nil {
		t.Fatal("unknown transfer protocol key should be rejected")
	}
}
