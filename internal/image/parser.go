package image

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type FromInst struct {
	Base string
}

func (f FromInst) Type() InstructionType { return InstructionFrom }

type AptInst struct {
	Packages []string
}

func (a AptInst) Type() InstructionType { return InstructionApt }

type PipInst struct {
	Packages []string
}

func (p PipInst) Type() InstructionType { return InstructionPip }

type EnvInst struct {
	Key   string
	Value string
}

func (e EnvInst) Type() InstructionType { return InstructionEnv }

type WorkdirInst struct {
	WorkDir string
}

func (w WorkdirInst) Type() InstructionType { return InstructionWorkdir }

type CopyInst struct {
	Source string
	Target string
}

func (c CopyInst) Type() InstructionType { return InstructionCopy }

type RunInst struct {
	Command string
}

func (r RunInst) Type() InstructionType { return InstructionRun }

type CmdInst struct {
	Command []string
}

func (c CmdInst) Type() InstructionType { return InstructionCmd }

func ParseCboxfile(r io.Reader) (*ImageManifest, error) {
	manifest := &ImageManifest{
		Env: make(map[string]string),
	}

	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToUpper(parts[0])
		args := ""
		if len(parts) > 1 {
			args = strings.TrimSpace(parts[1])
		}

		switch InstructionType(cmd) {
		case InstructionFrom:
			manifest.Base = args

		case InstructionApt:
			pkgs := strings.Fields(args)
			manifest.Apt = append(manifest.Apt, pkgs...)

		case InstructionPip:
			pkgs := strings.Fields(args)
			manifest.Pip = append(manifest.Pip, pkgs...)

		case InstructionEnv:
			kv := strings.SplitN(args, "=", 2)
			if len(kv) == 2 {
				manifest.Env[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			} else {
				fields := strings.Fields(args)
				if len(fields) >= 2 {
					manifest.Env[fields[0]] = fields[1]
				}
			}

		case InstructionWorkdir:
			manifest.WorkDir = args

		case InstructionCopy:
			fields := strings.Fields(args)
			if len(fields) < 2 {
				return nil, fmt.Errorf("line %d: COPY requires source and target", lineNum)
			}
			manifest.Copies = append(manifest.Copies, CopySpec{
				Source: fields[0],
				Target: fields[1],
			})

		case InstructionRun:
			manifest.Runs = append(manifest.Runs, args)

		case InstructionCmd:
			if strings.HasPrefix(args, "[") && strings.HasSuffix(args, "]") {
				var cmdList []string
				if err := json.Unmarshal([]byte(args), &cmdList); err == nil {
					manifest.Command = cmdList
				} else {
					manifest.Command = strings.Fields(args)
				}
			} else {
				manifest.Command = []string{"sh", "-c", args}
			}

		default:
			return nil, fmt.Errorf("line %d: unknown instruction %q", lineNum, cmd)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return manifest, nil
}

func ParseCboxfileFile(path string) (*ImageManifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseCboxfile(f)
}
