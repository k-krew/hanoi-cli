package ui

import (
	"encoding/json"
	"fmt"
	"io"

	"hanoi-cli/planner"
)

func RenderExplanation(w io.Writer, exp *planner.MoveExplanation, format string) {
	switch format {
	case "json":
		renderExplanationJSON(w, exp)
	default:
		renderExplanationText(w, exp)
	}
}

func renderExplanationText(w io.Writer, exp *planner.MoveExplanation) {
	_, _ = fmt.Fprintf(w, "\n--- Explanation for move #%d ---\n\n", exp.MoveNumber)

	_, _ = fmt.Fprintf(w, "Pod:    %s/%s\n", exp.PodNamespace, exp.PodName)
	if exp.PodOwner != "" {
		_, _ = fmt.Fprintf(w, "Owner:  %s\n", exp.PodOwner)
	}
	_, _ = fmt.Fprintf(w, "CPU:    %dm    MEM: %dMi\n", exp.PodCPU, exp.PodMem/(1024*1024))
	_, _ = fmt.Fprintf(w, "Move:   %s -> %s\n\n", exp.FromNode, exp.ToNode)

	if exp.IsSimulation {
		_, _ = fmt.Fprintf(w, "Source node (%s): FAILED (simulated)\n", exp.FromNode)
	} else {
		_, _ = fmt.Fprintf(w, "Source node (%s) utilization: CPU %.1f%%, MEM %.1f%%\n",
			exp.FromNode, exp.FromCPU*100, exp.FromMem*100)
	}
	_, _ = fmt.Fprintf(w, "Cluster score: %.1f%% -> %.1f%%\n\n", exp.ScoreBefore, exp.ScoreAfter)

	_, _ = fmt.Fprintln(w, "Candidate nodes:")
	for _, c := range exp.Candidates {
		if !c.Eligible {
			_, _ = fmt.Fprintf(w, "  %-20s REJECTED: %s\n", c.Name, c.Reason)
			continue
		}
		label := "eligible"
		if c.Name == exp.ToNode {
			label = "CHOSEN  "
		}
		affinityNote := affinityAnnotation(c)
		_, _ = fmt.Fprintf(w, "  %-20s %s CPU: %.1f%% -> %.1f%%  MEM: %.1f%% -> %.1f%%  score: %.1f%%%s\n",
			c.Name, label, c.CPUBefore*100, c.CPUAfter*100, c.MemBefore*100, c.MemAfter*100, c.ScoreIfChosen, affinityNote)
	}

	_, _ = fmt.Fprintln(w)
	chosen := findChosen(exp)
	if chosen != nil {
		best := true
		for _, c := range exp.Candidates {
			if c.Eligible && c.Name != exp.ToNode && c.ScoreIfChosen < chosen.ScoreIfChosen {
				best = false
				break
			}
		}
		if best {
			_, _ = fmt.Fprintf(w, "Verdict: %s produces the lowest imbalance score (%.1f%%) among all eligible nodes.\n",
				exp.ToNode, chosen.ScoreIfChosen)
		} else {
			_, _ = fmt.Fprintf(w, "Verdict: %s was selected with score %.1f%%.\n",
				exp.ToNode, chosen.ScoreIfChosen)
		}
	}
}

func renderExplanationJSON(w io.Writer, exp *planner.MoveExplanation) {
	type candidateJSON struct {
		Name                        string  `json:"name"`
		Eligible                    bool    `json:"eligible"`
		Reason                      string  `json:"reject_reason,omitempty"`
		CPUBefore                   float64 `json:"cpu_before_percent,omitempty"`
		CPUAfter                    float64 `json:"cpu_after_percent,omitempty"`
		MemBefore                   float64 `json:"mem_before_percent,omitempty"`
		MemAfter                    float64 `json:"mem_after_percent,omitempty"`
		ScoreIfChosen               float64 `json:"score_if_chosen,omitempty"`
		Chosen                      bool    `json:"chosen,omitempty"`
		PodAffinityOK               *bool   `json:"pod_affinity_ok,omitempty"`
		AntiAffinityOK              *bool   `json:"anti_affinity_ok,omitempty"`
		PreferredAntiAffinityWeight int32   `json:"preferred_anti_affinity_weight,omitempty"`
	}
	type explainJSON struct {
		MoveNumber   int             `json:"move_number"`
		Pod          string          `json:"pod"`
		Namespace    string          `json:"namespace"`
		Owner        string          `json:"owner,omitempty"`
		PodCPU       int64           `json:"pod_cpu_milli"`
		PodMem       int64           `json:"pod_mem_bytes"`
		From         string          `json:"from"`
		To           string          `json:"to"`
		IsSimulation bool            `json:"is_simulation"`
		FromCPU      float64         `json:"source_cpu_percent"`
		FromMem      float64         `json:"source_mem_percent"`
		ScoreBefore  float64         `json:"score_before"`
		ScoreAfter   float64         `json:"score_after"`
		Candidates   []candidateJSON `json:"candidates"`
	}

	out := explainJSON{
		MoveNumber:   exp.MoveNumber,
		Pod:          exp.PodName,
		Namespace:    exp.PodNamespace,
		Owner:        exp.PodOwner,
		PodCPU:       exp.PodCPU,
		PodMem:       exp.PodMem,
		From:         exp.FromNode,
		To:           exp.ToNode,
		IsSimulation: exp.IsSimulation,
		FromCPU:      exp.FromCPU * 100,
		FromMem:      exp.FromMem * 100,
		ScoreBefore:  exp.ScoreBefore,
		ScoreAfter:   exp.ScoreAfter,
	}
	for _, c := range exp.Candidates {
		cj := candidateJSON{
			Name:     c.Name,
			Eligible: c.Eligible,
			Chosen:   c.Name == exp.ToNode,
		}
		if !c.Eligible {
			cj.Reason = c.Reason
		} else {
			cj.CPUBefore = c.CPUBefore * 100
			cj.CPUAfter = c.CPUAfter * 100
			cj.MemBefore = c.MemBefore * 100
			cj.MemAfter = c.MemAfter * 100
			cj.ScoreIfChosen = c.ScoreIfChosen
			if c.HasPodAffinity {
				cj.PodAffinityOK = &c.PodAffinityOK
			}
			if c.HasAntiAffinity {
				cj.AntiAffinityOK = &c.AntiAffinityOK
			}
			cj.PreferredAntiAffinityWeight = c.PreferredAntiAffinityWeight
		}
		out.Candidates = append(out.Candidates, cj)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func affinityAnnotation(c planner.NodeCandidate) string {
	var parts []string
	if c.HasPodAffinity {
		if c.PodAffinityOK {
			parts = append(parts, "pod-affinity: ok")
		} else {
			parts = append(parts, "pod-affinity: FAIL")
		}
	}
	if c.HasAntiAffinity {
		if c.AntiAffinityOK {
			parts = append(parts, "anti-affinity: ok")
		} else {
			parts = append(parts, "anti-affinity: FAIL")
		}
	}
	if c.PreferredAntiAffinityWeight > 0 {
		parts = append(parts, fmt.Sprintf("preferred-anti-affinity: VIOLATED weight=%d", c.PreferredAntiAffinityWeight))
	}
	if len(parts) == 0 {
		return ""
	}
	result := "  ("
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result + ")"
}

func findChosen(exp *planner.MoveExplanation) *planner.NodeCandidate {
	for i := range exp.Candidates {
		if exp.Candidates[i].Name == exp.ToNode {
			return &exp.Candidates[i]
		}
	}
	return nil
}
