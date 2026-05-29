package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/CemAkan/pastaay/pkg/config"
	"gopkg.in/yaml.v3"
)

// reAnchor/reAlias detect YAML anchors and aliases in BOTH block style and flow style
var (
	reAnchor = regexp.MustCompile(`(?m)(^|[\s\:\,\-\[\{])&[A-Za-z0-9_\-]+`)
	reAlias  = regexp.MustCompile(`(?m)(^|[\s\:\,\-\[\{])\*[A-Za-z0-9_\-]+`)
)

const maxPlanPayloadBytes = 1 << 20

func PlanFromRequest(r *http.Request) (PlanResult, error) {
	if r.Body == nil {
		return PlanResult{}, fmt.Errorf("empty body")
	}
	defer r.Body.Close()

	limited := io.LimitReader(r.Body, maxPlanPayloadBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return PlanResult{}, err
	}
	if int64(len(body)) > maxPlanPayloadBytes {
		return PlanResult{}, fmt.Errorf("payload exceeds %d bytes", maxPlanPayloadBytes)
	}

	// Reject alias bombs prior to decode.
	if isYAMLLikely(body) && hasAliasMarkers(body) {
		return PlanResult{}, fmt.Errorf("YAML aliases are not permitted")
	}

	var cfg config.PastaayConfig
	ct := strings.ToLower(r.Header.Get("Content-Type"))

	if strings.Contains(ct, "json") {
		if err := json.Unmarshal(body, &cfg); err != nil {
			return PlanResult{}, err
		}
	} else {
		if err := yaml.Unmarshal(body, &cfg); err != nil {
			return PlanResult{}, err
		}
	}

	return Analyze(&cfg), nil
}

func isYAMLLikely(body []byte) bool {
	s := strings.TrimSpace(string(body))
	return !strings.HasPrefix(s, "{") && !strings.HasPrefix(s, "[")
}

func hasAliasMarkers(body []byte) bool {
	return reAnchor.Match(body) && reAlias.Match(body)
}
