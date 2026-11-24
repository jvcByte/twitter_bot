package generation

import "testing"

// Available gates all AI-written content, so a missing provider env key here
// silently downgrades posts to raw headlines.
func TestAvailable(t *testing.T) {
	keys := []string{"GROQ_API_KEY", "OPENROUTER_API_KEY", "GEMINI_API_KEY"}

	for _, key := range keys {
		for _, k := range keys {
			t.Setenv(k, "")
		}
		if Available() {
			t.Fatal("Available() = true with no keys set")
		}
		t.Setenv(key, "test-key")
		if !Available() {
			t.Fatalf("Available() = false with %s set", key)
		}
	}
}

func TestProvidersPreferGroq(t *testing.T) {
	if providers[0].envKey != "GROQ_API_KEY" {
		t.Fatalf("primary provider = %q, want Groq", providers[0].name)
	}
}
