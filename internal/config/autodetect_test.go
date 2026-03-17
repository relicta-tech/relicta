package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestAutoDetectAI_OpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-key")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_KEY", "")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("OLLAMA_HOST", "")

	l := &Loader{v: viper.New()}
	l.autoDetectAI()

	if l.v.GetString("ai.provider") != "openai" {
		t.Errorf("expected provider openai, got %s", l.v.GetString("ai.provider"))
	}
	if !l.v.GetBool("ai.enabled") {
		t.Error("expected ai.enabled to be true")
	}
	if l.v.GetString("ai.model") != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %s", l.v.GetString("ai.model"))
	}
}

func TestAutoDetectAI_Anthropic(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_KEY", "")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("OLLAMA_HOST", "")

	l := &Loader{v: viper.New()}
	l.autoDetectAI()

	if l.v.GetString("ai.provider") != "anthropic" {
		t.Errorf("expected provider anthropic, got %s", l.v.GetString("ai.provider"))
	}
	if l.v.GetString("ai.model") != "claude-sonnet-4" {
		t.Errorf("expected model claude-sonnet-4, got %s", l.v.GetString("ai.model"))
	}
}

func TestAutoDetectAI_Gemini(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("AZURE_OPENAI_KEY", "")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("OLLAMA_HOST", "")

	l := &Loader{v: viper.New()}
	l.autoDetectAI()

	if l.v.GetString("ai.provider") != "gemini" {
		t.Errorf("expected provider gemini, got %s", l.v.GetString("ai.provider"))
	}
	if l.v.GetString("ai.model") != "gemini-2.0-flash-exp" {
		t.Errorf("expected model gemini-2.0-flash-exp, got %s", l.v.GetString("ai.model"))
	}
}

func TestAutoDetectAI_AzureOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_KEY", "azure-key")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://myinstance.openai.azure.com")
	t.Setenv("OLLAMA_HOST", "")

	l := &Loader{v: viper.New()}
	l.autoDetectAI()

	if l.v.GetString("ai.provider") != "azure-openai" {
		t.Errorf("expected provider azure-openai, got %s", l.v.GetString("ai.provider"))
	}
	if l.v.GetString("ai.model") != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", l.v.GetString("ai.model"))
	}
}

func TestAutoDetectAI_AzureOpenAI_MissingEndpoint(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_KEY", "azure-key")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("OLLAMA_HOST", "")

	l := &Loader{v: viper.New()}
	l.autoDetectAI()

	// Azure without endpoint should not be selected
	if l.v.GetString("ai.provider") != "" {
		t.Errorf("expected no provider, got %s", l.v.GetString("ai.provider"))
	}
}

func TestAutoDetectAI_Ollama(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_KEY", "")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("OLLAMA_HOST", "http://localhost:11434")

	l := &Loader{v: viper.New()}
	l.autoDetectAI()

	if l.v.GetString("ai.provider") != "ollama" {
		t.Errorf("expected provider ollama, got %s", l.v.GetString("ai.provider"))
	}
	if l.v.GetString("ai.model") != "llama3.2" {
		t.Errorf("expected model llama3.2, got %s", l.v.GetString("ai.model"))
	}
}

func TestAutoDetectAI_NoProviders(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_KEY", "")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("OLLAMA_HOST", "")

	l := &Loader{v: viper.New()}
	l.autoDetectAI()

	if l.v.GetBool("ai.enabled") {
		t.Error("expected ai.enabled to be false when no providers detected")
	}
}

func TestAutoDetectAI_MultipleProviders_PrefersOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_KEY", "")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("OLLAMA_HOST", "")

	l := &Loader{v: viper.New()}
	l.autoDetectAI()

	if l.v.GetString("ai.provider") != "openai" {
		t.Errorf("expected openai as priority provider, got %s", l.v.GetString("ai.provider"))
	}
}
