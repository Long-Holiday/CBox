package colab

import (
	"encoding/json"
	"strings"

	cboxErr "cbox/internal/errors"
)

type SessionInfo struct {
	Session string `json:"session"`
	GPU     string `json:"gpu"`
	State   string `json:"state"`
}

type StatusInfo struct {
	Session string `json:"session"`
	GPU     string `json:"gpu"`
	State   string `json:"state"`
	Alive   bool   `json:"alive"`
}

func ParseSessions(output []byte) ([]SessionInfo, error) {
	var sessions []SessionInfo
	if err := json.Unmarshal(output, &sessions); err == nil {
		return sessions, nil
	}

	// Fallback to line-based parsing
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "SESSION") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			s := SessionInfo{
				Session: fields[0],
			}
			if len(fields) >= 2 {
				s.GPU = fields[1]
			}
			if len(fields) >= 3 {
				s.State = fields[2]
			}
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

func ParseStatus(output []byte) (*StatusInfo, error) {
	var s StatusInfo
	if err := json.Unmarshal(output, &s); err == nil {
		return &s, nil
	}

	outStr := string(output)
	info := &StatusInfo{
		Alive: true,
	}

	if strings.Contains(strings.ToLower(outStr), "not found") ||
		strings.Contains(strings.ToLower(outStr), "no active session") ||
		strings.Contains(strings.ToLower(outStr), "stopped") {
		info.Alive = false
		info.State = "stopped"
		return info, nil
	}

	lines := strings.Split(outStr, "\n")
	for _, l := range lines {
		parts := strings.SplitN(l, ":", 2)
		if len(parts) == 2 {
			k := strings.ToLower(strings.TrimSpace(parts[0]))
			v := strings.TrimSpace(parts[1])
			switch k {
			case "session":
				info.Session = v
			case "gpu":
				info.GPU = v
			case "state", "status":
				info.State = v
			}
		}
	}

	return info, nil
}

func ParseCLIError(binary string, res *CommandResult) error {
	if res.ExitCode == 0 {
		return nil
	}

	out := strings.ToLower(string(res.Stdout) + " " + string(res.Stderr))

	if strings.Contains(out, "gpu not available") ||
		strings.Contains(out, "quota exceeded") ||
		strings.Contains(out, "cannot allocate") ||
		strings.Contains(out, "out of memory") {
		return &cboxErr.AllocationError{
			Reason: strings.TrimSpace(string(res.Stderr)),
		}
	}

	if strings.Contains(out, "auth") ||
		strings.Contains(out, "login") ||
		strings.Contains(out, "permission denied") ||
		strings.Contains(out, "unauthorized") {
		return &cboxErr.AuthenticationError{
			Provider: "colab",
			Message:  strings.TrimSpace(string(res.Stderr)),
		}
	}

	return &cboxErr.CommandError{
		Command:  binary,
		ExitCode: res.ExitCode,
		Stdout:   string(res.Stdout),
		Stderr:   string(res.Stderr),
	}
}
