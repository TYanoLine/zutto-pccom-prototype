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

func TestHAKATAGeneratedContentLoggingDefaultsOn(t *testing.T) {
	t.Setenv("DEBUG_LOG_HAKATA_GENERATED", "")
	if !Load().DebugLogHAKATAGenerated {
		t.Fatal("HAKATA generator-evaluation content should log by default")
	}
}

func TestHAKATAGeneratedContentLoggingCanBeDisabled(t *testing.T) {
	t.Setenv("DEBUG_LOG_HAKATA_GENERATED", "0")
	if Load().DebugLogHAKATAGenerated {
		t.Fatal("HAKATA generated content logging should be disableable")
	}
}

func TestHakataPromptTraceFlagDefaultsOnAndCanBeDisabled(t *testing.T) {
	t.Setenv("DEBUG_HAKATA_LLM_TRACE", "")
	if !Load().DebugHakataLLMTrace { t.Fatal("HAKATA evaluation trace should be enabled without a debug token") }
	t.Setenv("DEBUG_HAKATA_LLM_TRACE", "0")
	if Load().DebugHakataLLMTrace { t.Fatal("HAKATA trace flag should be disableable") }
}

func TestHAKATAFreeformBodyDefaultAndOptOut(t *testing.T) {
	t.Setenv("HAKATA_FREEFORM_BODY", "")
	if !Load().HakataFreeformBody { t.Fatal("title-led body experiment should default on in HAKATA") }
	t.Setenv("HAKATA_FREEFORM_BODY", "0")
	if Load().HakataFreeformBody { t.Fatal("title-led body experiment should be reversible") }
}

func TestAzureOpenAIVerbosityDefaultsEmpty(t *testing.T) {
	t.Setenv("AZURE_OPENAI_VERBOSITY", "")
	if got := Load().AzureOpenAIVerbosity; got != "" {
		t.Fatalf("default verbosity override = %q, want empty", got)
	}
	t.Setenv("AZURE_OPENAI_VERBOSITY", "medium")
	if got := Load().AzureOpenAIVerbosity; got != "medium" {
		t.Fatalf("verbosity override = %q, want medium", got)
	}
}
