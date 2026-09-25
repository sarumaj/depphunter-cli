package findings

import "testing"

// The base scores the specification's own examples produce. Getting these wrong would
// put the wrong color on every bug in the city.
//
// Verifies: REQ-FND-018
func TestCVSSBaseScores(t *testing.T) {
	for vector, want := range map[string]float64{
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H": 9.8,
		"CVSS:3.0/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H": 9.8,
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H": 7.5,
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L": 5.3,
		"CVSS:3.1/AV:L/AC:L/PR:N/UI:R/S:U/C:H/I:H/A:H": 7.8,
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:C/C:L/I:L/A:N": 6.1,
		"CVSS:3.1/AV:N/AC:L/PR:L/UI:N/S:U/C:L/I:N/A:N": 4.3,
		"CVSS:3.1/AV:P/AC:H/PR:H/UI:R/S:U/C:N/I:N/A:N": 0,
	} {
		got, ok := scoreCVSS(vector)
		if !ok {
			t.Errorf("%s: not scored", vector)
			continue
		}
		if got != want {
			t.Errorf("%s: %.1f, want %.1f", vector, got, want)
		}
	}
}

func TestCVSSRejectsWhatItCannotScore(t *testing.T) {
	for _, vector := range []string{
		"",
		"CVSS:2.0/AV:N/AC:L/Au:N/C:P/I:P/A:P", // v2 uses other metrics
		"AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", // no version prefix
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/C:H/I:H/A:H",         // no scope
		"CVSS:3.1/AV:X/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",     // unknown metric value
		"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H", // v4 is not v3
	} {
		if score, ok := scoreCVSS(vector); ok {
			t.Errorf("%q scored %.1f", vector, score)
		}
	}
}

// Verifies: REQ-FND-018
func TestSeverityOfScore(t *testing.T) {
	for score, want := range map[float64]Severity{
		0: Info, 0.1: Low, 3.9: Low, 4: Medium, 6.9: Medium,
		7: High, 8.9: High, 9: Critical, 10: Critical,
	} {
		if got := severityOfScore(score); got != want {
			t.Errorf("%.1f: %q, want %q", score, got, want)
		}
	}
}
