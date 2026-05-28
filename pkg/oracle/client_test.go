package oracle

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAskOracle_DefaultTimeoutWiring(t *testing.T) {
	start := time.Now()
	_, err := AskOracle("unknown", "", "", "low", "ping", "")
	if err == nil {
		t.Fatal("expected unknown-provider error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("AskOracle did not respect default timeout")
	}
}

func TestAskOracleCtx_RespectsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := AskOracleCtx(ctx, "unknown", "", "", "low", "x", "")
	if err == nil {
		t.Fatal("expected error on cancelled context with bad provider")
	}
}

func TestExtractYAML_FenceVariants(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercase-yaml", "```yaml\nversion: 1\n```", "version: 1"},
		{"uppercase-yaml", "```YAML\nversion: 1\n```", "version: 1"},
		{"yml-fence", "```yml\nversion: 1\n```", "version: 1"},
		{"no-fence", "version: 1", ""},
		{"unclosed-fence", "```yaml\nversion: 1", ""},
		{"with-prefix", "Here is the config:\n```yaml\nversion: 1\npolicies: []\n```\nDone.", "version: 1\npolicies: []"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtractYAML(c.in)
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestTruncate_MultiByteBoundary(t *testing.T) {
	// cut at 5 should land on a rune boundary.
	s := "✨ " + strings.Repeat("x", 100)
	got := truncate(s, 5)
	if got == "" {
		t.Fatal("truncate dropped the entire string")
	}
	if !strings.Contains(got, "...(truncated)") {
		t.Fatal("truncate marker missing")
	}
}

func TestTruncate_ShortStringUnchanged(t *testing.T) {
	if got := truncate("hi", 100); got != "hi" {
		t.Fatalf("short string mutated: %q", got)
	}
}

func TestBuildSystemPrompt_HasIntensityClause(t *testing.T) {
	sp := buildSystemPrompt(buildIntensityGuide("nuke"))
	if !strings.Contains(sp, "MAXIMUM DESTRUCTION") {
		t.Fatal("nuke intensity not propagated into prompt")
	}
}

func TestExtractYAML_PreservesCasingAndAvoidsFullLower(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"```YAML\nVersion: 1\n```", "Version: 1"},
		{"```Yml\nFOO: BaR\n```", "FOO: BaR"},
		{"prefix\n```yaml\nfoo: \"BaR\"\n```\nsuffix", "foo: \"BaR\""},
	}
	for i, c := range cases {
		if got := ExtractYAML(c.in); got != c.want {
			t.Fatalf("case %d: got %q want %q", i, got, c.want)
		}
	}
}

func BenchmarkExtractYAML_LargeBody(b *testing.B) {
	body := strings.Repeat("noise line\n", 1024) +
		"```yaml\nversion: 1\npolicies: []\n```\n" +
		strings.Repeat("trailing noise\n", 1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractYAML(body)
	}
}
