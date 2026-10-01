package config

import "testing"

func TestAzureOpenAIDefaultModelUsesDeploymentName(t *testing.T) {
	t.Setenv("AZURE_OPENAI_MODEL", "")
	t.Setenv("AZURE_OPENAI_IMAGE_MODEL", "")

	cfg := Load()
	if cfg.AzureOpenAIModel != "zutto-pccom-gpt-6-luna" {
		t.Fatalf("AzureOpenAIModel=%q want deployment name %q", cfg.AzureOpenAIModel, "zutto-pccom-gpt-6-luna")
	}
	if cfg.AzureOpenAIImageModel != "gpt-image-1" {
		t.Fatalf("AzureOpenAIImageModel=%q want %q", cfg.AzureOpenAIImageModel, "gpt-image-1")
	}
}

func TestHistoricalReferencesDefaultOff(t *testing.T) {
	t.Setenv("HISTORICAL_REFERENCES_ENABLED", "")
	if Load().HistoricalReferencesEnabled {
		t.Fatal("historical references must default to OFF")
	}
}

func TestHistoricalReferencesCanBeEnabledAndDisabled(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "on", want: true},
		{value: "0", want: false},
		{value: "false", want: false},
		{value: "off", want: false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("HISTORICAL_REFERENCES_ENABLED", tc.value)
			if got := Load().HistoricalReferencesEnabled; got != tc.want {
				t.Fatalf("value=%q got=%v want=%v", tc.value, got, tc.want)
			}
		})
	}
}


func TestDebugDisableBBSTitleHistoricalVerificationDefaultOff(t *testing.T) {
	t.Setenv("DEBUG_DISABLE_BBS_TITLE_HISTORICAL_VERIFICATION", "")
	if Load().DebugDisableBBSTitleHistoricalVerification {
		t.Fatal("title historical verification debug bypass must default to OFF")
	}
}

func TestDebugDisableBBSTitleHistoricalVerificationCanBeEnabled(t *testing.T) {
	t.Setenv("DEBUG_DISABLE_BBS_TITLE_HISTORICAL_VERIFICATION", "1")
	if !Load().DebugDisableBBSTitleHistoricalVerification {
		t.Fatal("debug title historical verification bypass was not enabled")
	}
}


func TestDebugLogBBSArticleDetailsDefaultOff(t *testing.T) {
	t.Setenv("DEBUG_LOG_BBS_ARTICLE_DETAILS", "")
	if Load().DebugLogBBSArticleDetails {
		t.Fatal("Article Detail debug logging must default to OFF")
	}
}

func TestDebugLogBBSArticleDetailsCanBeEnabled(t *testing.T) {
	t.Setenv("DEBUG_LOG_BBS_ARTICLE_DETAILS", "1")
	if !Load().DebugLogBBSArticleDetails {
		t.Fatal("Article Detail debug logging was not enabled")
	}
}
