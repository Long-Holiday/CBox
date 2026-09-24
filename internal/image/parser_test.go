package image

import (
	"strings"
	"testing"
)

func TestParseCboxfile(t *testing.T) {
	cboxContent := `
FROM colab/python:3

APT git rsync libgl1

PIP torch torchvision
PIP mmengine mmcv

ENV PYTHONPATH=/workspace
WORKDIR /workspace

COPY requirements.txt /tmp/requirements.txt
RUN pip install -r /tmp/requirements.txt

CMD ["python", "train.py"]
`
	manifest, err := ParseCboxfile(strings.NewReader(cboxContent))
	if err != nil {
		t.Fatalf("failed to parse cboxfile: %v", err)
	}

	if manifest.Base != "colab/python:3" {
		t.Fatalf("expected base colab/python:3, got %s", manifest.Base)
	}

	if len(manifest.Apt) != 3 || manifest.Apt[0] != "git" || manifest.Apt[2] != "libgl1" {
		t.Fatalf("unexpected apt packages: %v", manifest.Apt)
	}

	if len(manifest.Pip) != 4 {
		t.Fatalf("expected 4 pip packages, got %v", manifest.Pip)
	}

	if manifest.Env["PYTHONPATH"] != "/workspace" {
		t.Fatalf("expected PYTHONPATH=/workspace, got %s", manifest.Env["PYTHONPATH"])
	}

	if manifest.WorkDir != "/workspace" {
		t.Fatalf("expected workdir /workspace, got %s", manifest.WorkDir)
	}

	if len(manifest.Copies) != 1 || manifest.Copies[0].Source != "requirements.txt" {
		t.Fatalf("unexpected copies: %v", manifest.Copies)
	}

	if len(manifest.Runs) != 1 || manifest.Runs[0] != "pip install -r /tmp/requirements.txt" {
		t.Fatalf("unexpected runs: %v", manifest.Runs)
	}

	if len(manifest.Command) != 2 || manifest.Command[1] != "train.py" {
		t.Fatalf("unexpected command: %v", manifest.Command)
	}
}
