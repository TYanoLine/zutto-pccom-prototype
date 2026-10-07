package config

import "testing"

func TestAzureOpenAIDefaultModelUsesDeploymentName(t *testing.T) {
	t.Setenv("AZURE_OPENAI_MODEL", "")

	cfg := Load()
	if cfg.AzureOpenAIModel != "zutto-pccom-gpt-6-luna" {
		t.Fatalf("AzureOpenAIModel=%q want deployment name %q", cfg.AzureOpenAIModel, "zutto-pccom-gpt-6-luna")
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

func TestGeneratedContentLoggingDefaultsOn(t *testing.T) {
	t.Setenv("DEBUG_LOG_GENERATED_CONTENT", "")
	if !Load().DebugLogGeneratedContent {
		t.Fatal("generated content should log by default")
	}
}

func TestGeneratedContentLoggingCanBeDisabled(t *testing.T) {
	t.Setenv("DEBUG_LOG_GENERATED_CONTENT", "0")
	if Load().DebugLogGeneratedContent {
		t.Fatal("generated content logging should be disableable")
	}
}

func TestGenerationTraceFlagDefaultsOnAndCanBeDisabled(t *testing.T) {
	t.Setenv("DEBUG_GENERATION_TRACE", "")
	if !Load().DebugGenerationTrace {
		t.Fatal("generation trace should be enabled by default")
	}
	t.Setenv("DEBUG_GENERATION_TRACE", "0")
	if Load().DebugGenerationTrace {
		t.Fatal("generation trace should be disableable")
	}
}

func TestGenerationFreeformBodyDefaultAndOptOut(t *testing.T) {
	t.Setenv("GENERATION_FREEFORM_BODY", "")
	if !Load().GenerationFreeformBody {
		t.Fatal("freeform body generation should default on")
	}
	t.Setenv("GENERATION_FREEFORM_BODY", "0")
	if Load().GenerationFreeformBody {
		t.Fatal("freeform body generation should be reversible")
	}
}

// The HAKATA-prefixed names were renamed and are no longer read. A deployment
// that still sets one of them (for example to 0) silently gets the default (on)
// until it moves to the new name, so this records that they have no effect.
func TestFormerHAKATAEnvironmentNamesAreIgnored(t *testing.T) {
	for _, tc := range []struct {
		name   string
		former string
		field  func(Config) bool
	}{
		{"generated content", "DEBUG_LOG_HAKATA_GENERATED", func(cfg Config) bool { return cfg.DebugLogGeneratedContent }},
		{"generation trace", "DEBUG_HAKATA_LLM_TRACE", func(cfg Config) bool { return cfg.DebugGenerationTrace }},
		{"freeform body", "HAKATA_FREEFORM_BODY", func(cfg Config) bool { return cfg.GenerationFreeformBody }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.former, "0")
			if !tc.field(Load()) {
				t.Fatalf("%s=0 changed the configuration; the former names must no longer be read", tc.former)
			}
		})
	}
}
