package template

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"text/template"
)

//go:embed sandbox.lima.yml
var sandboxTemplate string

// Params holds the values injected into the Lima template.
type Params struct {
	ProjectDir   string // mount at /workspace (non-branch mode only)
	GitURL       string // http://host.lima.internal:<port>/ (branch mode)
	Branch       string // branch to clone
	GitUserName  string // propagate host git user.name
	GitUserEmail string // propagate host git user.email
	APIKey       string
	CPUs         int
	Memory       string
	Disk         string
}

// Render renders the embedded Lima template with the given params and writes
// it to a temporary file, returning the file path. The caller is responsible
// for removing the file.
func Render(p Params) (string, error) {
	tmpl, err := template.New("sandbox").Parse(sandboxTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	f, err := os.CreateTemp("", "sandbox-*.lima.yml")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(buf.Bytes()); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("writing template: %w", err)
	}

	return f.Name(), nil
}
