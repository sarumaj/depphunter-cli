package findings

import (
	"math"
	"strings"
)

// CVSS base metrics, by vector abbreviation. Scope decides which table privileges
// required is read from, which is the only place the two differ.
var (
	attackVector       = map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2}
	attackComplexity   = map[string]float64{"L": 0.77, "H": 0.44}
	privilegesUnscoped = map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	privilegesScoped   = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.5}
	userInteraction    = map[string]float64{"N": 0.85, "R": 0.62}
	impact             = map[string]float64{"H": 0.56, "L": 0.22, "N": 0}
)

// scoreCVSS computes the CVSS v3 base score of a vector string, as the specification
// defines it. Advisories carry the vector far more often than a severity word, so this
// is usually what decides how serious a finding looks on the map. ok is false for a
// vector that is not v3 or is missing a base metric.
//
// Implements: REQ-FND-018
func scoreCVSS(vector string) (float64, bool) {
	m := map[string]string{}
	parts := strings.Split(strings.TrimSpace(vector), "/")
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "CVSS:3") {
		return 0, false
	}
	for _, p := range parts[1:] {
		k, v, ok := strings.Cut(p, ":")
		if ok {
			m[k] = v
		}
	}
	av, ok1 := attackVector[m["AV"]]
	ac, ok2 := attackComplexity[m["AC"]]
	ui, ok3 := userInteraction[m["UI"]]
	c, ok4 := impact[m["C"]]
	i, ok5 := impact[m["I"]]
	a, ok6 := impact[m["A"]]
	changed := m["S"] == "C"
	pr, ok7 := privilegesUnscoped[m["PR"]]
	if changed {
		pr, ok7 = privilegesScoped[m["PR"]]
	}
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7) || (m["S"] != "C" && m["S"] != "U") {
		return 0, false
	}

	iss := 1 - (1-c)*(1-i)*(1-a)
	var imp float64
	if changed {
		imp = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	} else {
		imp = 6.42 * iss
	}
	if imp <= 0 {
		return 0, true
	}
	exp := 8.22 * av * ac * pr * ui
	score := imp + exp
	if changed {
		score *= 1.08
	}
	return roundUp(math.Min(score, 10)), true
}

// roundUp is the specification's own rounding: to one decimal, always upwards, without
// the floating-point surprises a plain math.Ceil on x*10 produces.
func roundUp(x float64) float64 {
	n := int(math.Round(x * 100000))
	if n%10000 == 0 {
		return float64(n) / 100000
	}
	return (math.Floor(float64(n)/10000) + 1) / 10
}

// severityOfScore is the qualitative rating the specification attaches to a score.
func severityOfScore(score float64) Severity {
	switch {
	case score >= 9:
		return Critical
	case score >= 7:
		return High
	case score >= 4:
		return Medium
	case score > 0:
		return Low
	}
	return Info
}
