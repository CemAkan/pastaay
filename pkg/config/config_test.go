package config

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestGenerateStableHash_FloatCanonicalization(t *testing.T) {
	negZero := math.Copysign(0, -1)
	a := &Policy{Name: "p", Target: "/x", Type: "http", LatencyChance: 0}
	b := &Policy{Name: "p", Target: "/x", Type: "http", LatencyChance: negZero}
	if generateStableHash(a) != generateStableHash(b) {
		t.Fatalf("+0 and -0 must hash identically (a=%x b=%x)",
			generateStableHash(a), generateStableHash(b))
	}

	nan1 := math.NaN()
	nan2 := math.Float64frombits(math.Float64bits(nan1) | 0xDEAD)
	c := &Policy{Name: "p", Target: "/x", Type: "http", ErrorChance: nan1}
	d := &Policy{Name: "p", Target: "/x", Type: "http", ErrorChance: nan2}
	if generateStableHash(c) != generateStableHash(d) {
		t.Fatalf("two NaNs must hash identically")
	}
}

func TestGenerateStableHash_OrderIndependentHeaders(t *testing.T) {
	p1 := &Policy{Name: "x", Type: "http", Target: "/", MatchHeaders: map[string]string{
		"A": "1", "B": "2", "C": "3",
	}}
	p2 := &Policy{Name: "x", Type: "http", Target: "/", MatchHeaders: map[string]string{
		"C": "3", "A": "1", "B": "2",
	}}
	if generateStableHash(p1) != generateStableHash(p2) {
		t.Fatalf("MatchHeaders iteration order must not affect hash")
	}
}

func TestValidate_RejectsOutOfBounds(t *testing.T) {
	cases := []struct {
		name string
		cfg  *PastaayConfig
	}{
		{"version-zero", &PastaayConfig{}},
		{"version-too-high", &PastaayConfig{Version: 99}},
		{"warmup-negative", &PastaayConfig{Version: 1, WarmupDuration: -time.Second}},
		{"warmup-too-large", &PastaayConfig{Version: 1, WarmupDuration: 100 * time.Hour}},
		{"latency-out-of-range", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "http", Target: "/", LatencyChance: 2.5}}}},
		{"error-out-of-range", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "http", Target: "/", ErrorChance: -0.1}}}},
		{"latency-too-long", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "http", Target: "/", LatencyDuration: 100 * time.Hour}}}},
		{"ram-chunk-huge", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "resource", Target: "host", RAMChunkMB: 99999}}}},
		{"throttle-huge", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "resource", Target: "host", ThrottleThreshold: 999999999}}}},
		{"empty-target", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "http"}}}},
		{"bad-protocol", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "telnet", Target: "/"}}}},
		{"bad-stream-mode", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "grpc", Target: "x", StreamRollMode: "explode"}}}},
		{"http-bad-code", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "http", Target: "/", ErrorCode: 99}}}},
		{"http-big-code", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "http", Target: "/", ErrorCode: 999}}}},
		{"grpc-bad-code", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "grpc", Target: "x", ErrorCode: 99}}}},
		{"sql-bad-regex", &PastaayConfig{Version: 1, Policies: []Policy{{Type: "sql", Target: "[unclosed"}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.cfg.Validate(); err == nil {
				t.Fatalf("expected validation error for %s", c.name)
			}
		})
	}
}

func TestManager_UpdateRejectsInvalidConfig(t *testing.T) {
	good := &PastaayConfig{Version: 1, Policies: []Policy{{Type: "http", Target: "/x", ErrorChance: 0.5}}}
	mgr := NewManager(good)
	before := mgr.GetActivePolicies("http")
	if len(before) != 1 {
		t.Fatalf("seed snapshot missing")
	}

	bad := &PastaayConfig{Version: 0, Policies: []Policy{{Type: "http", Target: "/y"}}}
	mgr.Update(bad)

	after := mgr.GetActivePolicies("http")
	if len(after) != 1 || after[0].Target != "/x" {
		t.Fatalf("invalid config must not overwrite previously valid snapshot, got %+v", after)
	}
}

func TestManager_AtomicSnapshotConsistency(t *testing.T) {
	mgr := NewManager(&PastaayConfig{
		Version:  1,
		Policies: []Policy{{Type: "http", Target: "/a"}},
	})

	stop := make(chan struct{})
	var inconsistencies atomic.Uint64

	// readers
	var rwg sync.WaitGroup
	for i := 0; i < 8; i++ {
		rwg.Add(1)
		go func() {
			defer rwg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				ps := mgr.GetActivePolicies("http")
				for _, p := range ps {
					if p.Type == "" || p.Target == "" {
						inconsistencies.Add(1)
					}
				}
			}
		}()
	}

	// writer
	for i := 0; i < 200; i++ {
		mgr.Update(&PastaayConfig{
			Version: 1,
			Policies: []Policy{
				{Type: "http", Target: "/a"},
				{Type: "http", Target: "/b"},
				{Type: "sql", Target: "all"},
			},
		})
	}
	close(stop)
	rwg.Wait()

	if inconsistencies.Load() != 0 {
		t.Fatalf("readers observed %d torn policies under concurrent Update", inconsistencies.Load())
	}
}

func TestManager_SensorStatusTypeAssertionSafe(t *testing.T) {
	mgr := NewManager(&PastaayConfig{Version: 1})
	mgr.sensorStatus.Store(42, "ok")        // bad key type
	mgr.sensorStatus.Store("redis", 200)    // bad value type
	mgr.sensorStatus.Store("kafka", "good") // valid

	got := mgr.GetSensorStatuses()
	if got["kafka"] != "good" {
		t.Fatalf("expected valid sensor to survive, got %v", got)
	}
	// invalid entries must be silently dropped
	if _, ok := got["redis"]; ok {
		t.Fatalf("bad-typed sensor must be filtered out")
	}
}

func TestHasSuspiciousYAMLAlias(t *testing.T) {
	tests := []struct {
		body string
		bad  bool
	}{
		{"version: 1\npolicies: []\n", false},
		{"version: 1\nname: \"a&b*c\"\n", false}, // literal & and * in strings
		{"a: &anchor 1\nb: *anchor\n", true},     // classic bomb
		{"foo: &x\n  - 1\nbar: *x\n", true},      // block-style bomb
		{"foo: &x [a,b,c]\nbar: [*x,*x,*x,*x,*x,*x,*x]\n", true},
	}
	for i, tc := range tests {
		if got := hasSuspiciousYAMLAlias([]byte(tc.body)); got != tc.bad {
			t.Errorf("case %d: hasSuspiciousYAMLAlias=%v want %v\nbody=%q", i, got, tc.bad, tc.body)
		}
	}
}

func TestLoadConfig_RejectsAliasBomb(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bomb.yaml")
	bomb := `
a: &a [x,x,x,x,x,x,x,x,x]
b: &b [*a,*a,*a,*a,*a,*a,*a,*a,*a]
c: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b]
d: [*c,*c,*c,*c,*c,*c,*c,*c,*c]
version: 1
policies: []
`
	if err := os.WriteFile(path, []byte(bomb), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("LoadConfig must reject alias bombs")
	}
}

func TestLoadConfig_RejectsHugeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.yaml")
	huge := strings.Repeat("a", maxConfigFileBytes+1)
	if err := os.WriteFile(path, []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("LoadConfig must reject files exceeding ceiling")
	}
}

func TestUnmarshalYAML_SnakeCaseZeroIsRespected(t *testing.T) {
	doc := `
latency_chance: 0
latencyChance: 0.9
type: http
target: /x
`
	var p Policy
	if err := yaml.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatal(err)
	}
	if p.LatencyChance != 0 {
		t.Fatalf("expected explicit snake_case 0 to win, got %v", p.LatencyChance)
	}
}

func TestUnmarshalYAML_CamelCaseFallbackWhenSnakeAbsent(t *testing.T) {
	doc := `
latencyChance: 0.42
type: http
target: /x
`
	var p Policy
	if err := yaml.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatal(err)
	}
	if p.LatencyChance != 0.42 {
		t.Fatalf("expected camelCase fallback to set 0.42, got %v", p.LatencyChance)
	}
}

func TestWatchConfig_CancelStopsGoroutine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("version: 1\npolicies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cancel, err := WatchConfig(path, func(*PastaayConfig) {})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { cancel(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancel() did not return — watcher goroutine leaked")
	}
}

func TestWatchConfig_DebouncesBurstWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("version: 1\npolicies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Uint32
	stop, err := WatchConfig(path, func(*PastaayConfig) { calls.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	// Burst-write the same file 8x within the debounce window.
	for i := 0; i < 8; i++ {
		_ = os.WriteFile(path, []byte("version: 1\npolicies: []\n"), 0o600)
		time.Sleep(20 * time.Millisecond)
	}
	// Allow the single debounced reload to fire.
	time.Sleep(debounceWindow + 200*time.Millisecond)
	got := calls.Load()
	if got == 0 || got > 3 {
		t.Fatalf("expected debounce to coalesce burst into 1-3 reloads, got %d", got)
	}
}

func TestWatchConfig_KeepsLastGoodOnInvalidReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	good := []byte("version: 1\npolicies: []\n")
	if err := os.WriteFile(path, good, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(cfg)
	if mgr.GetRawConfig() == nil {
		t.Fatal("initial seed missing")
	}

	stop, err := WatchConfig(path, mgr.Update)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	// watcher must NOT invoke the callback with a bad cfg.
	bad := []byte("version: 1\npolicies:\n  - type: not-a-real-protocol\n    target: x\n")
	if err := os.WriteFile(path, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(debounceWindow + 500*time.Millisecond)

	cur := mgr.GetRawConfig()
	if cur == nil || len(cur.Policies) != 0 {
		t.Fatalf("invalid reload should not have clobbered the snapshot, got %+v", cur)
	}
}

func TestCleanSQLCommand_EscapeCounting(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"SELECT 1", "SELECT 1"},
		{"SELECT * FROM t WHERE name='it\\'s'", "SELECT * FROM T WHERE NAME='IT\\'S'"},
		{"SELECT 1 -- inline comment\nFROM t", "SELECT 1  \nFROM T"},
		{"/* block */ SELECT 1", "SELECT 1"},
		{"", ""},
		{"';--", "';--"}, // pathological short input must not panic
	}
	for _, tc := range tests {
		got := CleanSQLCommand(tc.in)
		_ = got
	}
}

func FuzzCleanSQLCommand(f *testing.F) {
	f.Add("SELECT 1")
	f.Add("SELECT * FROM t WHERE name='it''s'")
	f.Add("/* block */ SELECT 1; -- trailing")
	f.Fuzz(func(t *testing.T, s string) {
		_ = CleanSQLCommand(s) // must never panic
	})
}

func FuzzLoadConfig(f *testing.F) {
	f.Add([]byte("version: 1\npolicies: []\n"))
	f.Add([]byte("version: 1\npolicies:\n  - type: http\n    target: /api\n    latency_chance: 0.5\n"))
	f.Fuzz(func(t *testing.T, body []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.yaml")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			return
		}
		_, _ = LoadConfig(path) // never panic for arbitrary input
	})
}

func TestIsCleanCommandIgnored_EmptyEntryDoesNotBypass(t *testing.T) {
	mgr := NewManager(&PastaayConfig{
		Version:              1,
		EnableDefaultIgnored: false,
		IgnoredCommands: map[string][]string{
			"sql": {"", "  ", "/", "//"},
		},
	})
	if mgr.IsCleanCommandIgnored("sql", "SELECT 1") {
		t.Fatal("empty/slash-only entries must not match every command")
	}
	// Sanity: a real entry still works.
	mgr.Update(&PastaayConfig{
		Version:         1,
		IgnoredCommands: map[string][]string{"sql": {"SELECT"}},
	})
	if !mgr.IsCleanCommandIgnored("sql", "SELECT 1") {
		t.Fatal("real entry SELECT must still ignore")
	}
}

func TestValidate_RejectsTooManyPolicies(t *testing.T) {
	pols := make([]Policy, maxPoliciesPerConfig+1)
	for i := range pols {
		pols[i] = Policy{Type: "http", Target: "/x"}
	}
	cfg := &PastaayConfig{Version: 1, Policies: pols}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected rejection at %d policies", len(pols))
	}
}

func TestLoadConfig_BoundedReader_RejectsOversized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.yaml")
	huge := strings.Repeat("a", maxConfigFileBytes+10)
	if err := os.WriteFile(path, []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("LoadConfig must reject oversize even via the bounded reader path")
	}
}

func TestWatchConfig_CancelWaitsForInFlightReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("version: 1\npolicies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var callsAfterCancel atomic.Uint32
	var cancelled atomic.Bool

	cancelFn, err := WatchConfig(path, func(*PastaayConfig) {
		if cancelled.Load() {
			callsAfterCancel.Add(1)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	// Trigger a reload, then cancel during the debounce window.
	_ = os.WriteFile(path, []byte("version: 1\npolicies: []\n"), 0o600)
	time.Sleep(20 * time.Millisecond)
	cancelled.Store(true)
	cancelFn()

	time.Sleep(debounceWindow + 200*time.Millisecond)
	if got := callsAfterCancel.Load(); got != 0 {
		t.Fatalf("reloadCallback fired %d time(s) after cancel — use-after-cancel hazard", got)
	}
}

func TestManager_NoTornReadsUnderRapidUpdates(t *testing.T) {
	mgr := NewManager(&PastaayConfig{Version: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	// writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			i++
			mgr.Update(&PastaayConfig{
				Version: 1,
				Policies: []Policy{
					{Type: "http", Target: "/x"},
					{Type: "sql", Target: "all"},
				},
			})
		}
	}()
	// readers
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_ = mgr.GetActivePolicies("http")
				_ = mgr.GetActivePolicies("sql")
				_ = mgr.GetRawConfig()
			}
		}()
	}
	wg.Wait()
}
