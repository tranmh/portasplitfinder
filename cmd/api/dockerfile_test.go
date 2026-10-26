package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestDockerfileFinalStageRunsAsNonRoot checks that the image the API ships in
// drops root: the final stage must set a USER that is neither root nor uid 0,
// so a compromise of the network-facing process does not get root in the
// container.
func TestDockerfileFinalStageRunsAsNonRoot(t *testing.T) {
	f, err := os.Open("Dockerfile")
	if err != nil {
		t.Fatalf("open Dockerfile: %v", err)
	}
	defer f.Close()

	// Track the last USER directive of the last stage.
	user := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "FROM":
			user = ""
		case "USER":
			if len(fields) > 1 {
				user = fields[1]
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	if user == "" {
		t.Fatalf("final stage has no USER directive: the API runs as root")
	}
	name := strings.SplitN(user, ":", 2)[0]
	if name == "root" || name == "0" {
		t.Fatalf("final stage USER is %q: the API runs as root", user)
	}
}
