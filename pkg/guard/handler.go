package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CemAkan/pastaay/pkg/config"
	"gopkg.in/yaml.v3"
)

const maxPlanPayloadBytes = 1 << 20 // 1MB Safety Cap

func PlanFromRequest(r *http.Request) (PlanResult, error) {
	if r.Body == nil {
		return PlanResult{}, fmt.Errorf("empty body")
	}
	defer r.Body.Close()

	body, err := io.ReadAll(io.LimitReader(r.Body, maxPlanPayloadBytes))
	if err != nil {
		return PlanResult{}, err
	}

	var cfg config.PastaayConfig
	ct := strings.ToLower(r.Header.Get("Content-Type"))

	// JSON/YAML switch
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
