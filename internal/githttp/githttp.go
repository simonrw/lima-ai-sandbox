package githttp

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
)

// Handler returns an http.Handler that implements the git smart HTTP protocol
// for the repository at repoPath.
func Handler(repoPath string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/info/refs", func(w http.ResponseWriter, r *http.Request) {
		service := r.URL.Query().Get("service")
		if service != "git-upload-pack" && service != "git-receive-pack" {
			http.Error(w, "invalid service", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/x-"+service+"-advertisement")
		w.Header().Set("Cache-Control", "no-cache")

		// Write pkt-line service header
		header := "# service=" + service + "\n"
		fmt.Fprintf(w, "%04x%s0000", len(header)+4, header)

		cmd := exec.CommandContext(r.Context(), "git", service[4:], "--stateless-rpc", "--advertise-refs", repoPath)
		cmd.Stdout = w
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			// Headers already sent; best-effort
			return
		}
	})

	mux.HandleFunc("/git-upload-pack", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
		w.Header().Set("Cache-Control", "no-cache")

		cmd := exec.CommandContext(r.Context(), "git", "upload-pack", "--stateless-rpc", repoPath)
		cmd.Stdin = r.Body
		cmd.Stdout = w
		cmd.Stderr = nil
		cmd.Run() //nolint: errcheck
	})

	mux.HandleFunc("/git-receive-pack", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
		w.Header().Set("Cache-Control", "no-cache")

		cmd := exec.CommandContext(r.Context(), "git", "receive-pack", "--stateless-rpc", repoPath)
		cmd.Stdin = r.Body
		cmd.Stdout = w
		cmd.Stderr = nil
		cmd.Run() //nolint: errcheck
	})

	return mux
}

// ListenAndServe starts the git HTTP server on a random port.
// It returns the port number after the listener is ready.
// The server blocks until the context is cancelled or an error occurs.
func ListenAndServe(repoPath string) (port int, stop func(), err error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, nil, fmt.Errorf("listen: %w", err)
	}

	port = ln.Addr().(*net.TCPAddr).Port

	// Enable receive.denyCurrentBranch=updateInstead so pushes to checked-out branch work
	gitCfg := exec.Command("git", "-C", repoPath, "config", "receive.denyCurrentBranch", "updateInstead")
	if out, err := gitCfg.CombinedOutput(); err != nil {
		ln.Close()
		return 0, nil, fmt.Errorf("git config: %s: %w", strings.TrimSpace(string(out)), err)
	}

	srv := &http.Server{Handler: Handler(repoPath)}

	go srv.Serve(ln) //nolint: errcheck

	return port, func() { srv.Close() }, nil
}
