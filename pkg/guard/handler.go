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
