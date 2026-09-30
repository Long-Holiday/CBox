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

func isStoppedState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "stopped", "terminated", "lost":
		return true
	}
	return false
}

// Colab 0.7.x prints [name] endpoint | Hardware: T4 | ... | Status: IDLE.
func parseSessionLine(line string) (SessionInfo, bool) {
	if !strings.HasPrefix(line, "[") || !strings.Contains(line, " | Hardware:") {
		return SessionInfo{}, false
	}
	end := strings.Index(line, "]")
	if end < 0 {
		return SessionInfo{}, false
	}
	info := SessionInfo{Session: line[1:end], State: "running"}
	if info.Session == "?" {
		fields := strings.Fields(strings.SplitN(line[end+1:], "|", 2)[0])
		if len(fields) == 0 {
			return SessionInfo{}, false
		}
		info.Session = fields[0]
	}
	for _, part := range strings.Split(line[end+1:], "|") {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			continue
		}
		value := strings.TrimSpace(kv[1])
		switch strings.ToLower(kv[0]) {
		case "hardware":
			if value != "CPU" && value != "NONE" {
				info.GPU = value
			}
		case "status":
			info.State = strings.ToLower(value)
		}
	}
	return info, true
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
		if info, ok := parseSessionLine(line); ok {
			sessions = append(sessions, info)
			continue
		}
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "SESSION") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			s := SessionInfo{
				Session: fields[0],
				GPU:     fields[1],
				State:   strings.ToLower(fields[2]),
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
		strings.Contains(strings.ToLower(outStr), "no active session") {
		info.Alive = false
		info.State = "stopped"
		return info, nil
	}

	lines := strings.Split(outStr, "\n")
	for _, l := range lines {
		if session, ok := parseSessionLine(strings.TrimSpace(l)); ok {
			info.Session = session.Session
			info.GPU = session.GPU
			info.State = session.State
			info.Alive = !isStoppedState(session.State)
			return info, nil
		}
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
				info.State = strings.ToLower(v)
			}
		}
	}
	info.Alive = !isStoppedState(info.State)

	return info, nil
}

func ParseCLIError(binary string, res *CommandResult) error {
	if res.ExitCode == 0 {
		return nil
	}

	out := strings.ToLower(string(res.Stdout) + " " + string(res.Stderr))

	if strings.Contains(out, "gpu not available") ||
		strings.Contains(out, "allocation refused") ||
		strings.Contains(out, "backend rejected accelerator") ||
		strings.Contains(out, "quota exceeded") ||
		strings.Contains(out, "cannot allocate") ||
		strings.Contains(out, "out of memory") {
		return &cboxErr.AllocationError{
			Reason: strings.TrimSpace(string(res.Stderr)),
		}
	}

	if strings.Contains(out, "authentication") ||
		strings.Contains(out, "invalid credentials") ||
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
