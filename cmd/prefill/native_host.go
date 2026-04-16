package prefill

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/lukaculjak/mak-cli/internal/prefills"
	"github.com/spf13/cobra"
)

type nativeRequest struct {
	Action   string `json:"action"`
	Password string `json:"password"`
}

type nativeResponse struct {
	Success  bool               `json:"success"`
	Projects []prefills.Project `json:"projects,omitempty"`
	Error    string             `json:"error,omitempty"`
}

// newNativeHostCmd implements Chrome Native Messaging protocol.
// Chrome starts the process, sends a 4-byte length-prefixed JSON message,
// and expects a 4-byte length-prefixed JSON response on stdout.
func newNativeHostCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "native-host",
		Short:  "Internal: Chrome Native Messaging host (do not call directly)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNativeHost(os.Stdin, os.Stdout)
		},
	}
}

func runNativeHost(r io.Reader, w io.Writer) error {
	msg, err := readNativeMessage(r)
	if err != nil {
		// If stdin closes cleanly (e.g. extension disconnected), exit silently.
		if err == io.EOF {
			return nil
		}
		return writeNativeResponse(w, nativeResponse{Success: false, Error: err.Error()})
	}

	var req nativeRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return writeNativeResponse(w, nativeResponse{Success: false, Error: "invalid request"})
	}

	switch req.Action {
	case "unlock":
		return handleUnlock(w, req.Password)
	default:
		return writeNativeResponse(w, nativeResponse{Success: false, Error: fmt.Sprintf("unknown action: %s", req.Action)})
	}
}

func handleUnlock(w io.Writer, password string) error {
	if !prefills.HasStore() {
		return writeNativeResponse(w, nativeResponse{Success: false, Error: "no prefill store found — run `mak prefill add` first"})
	}

	ok, err := prefills.VerifyPassword(password)
	if err != nil {
		return writeNativeResponse(w, nativeResponse{Success: false, Error: err.Error()})
	}
	if !ok {
		return writeNativeResponse(w, nativeResponse{Success: false, Error: "wrong master password"})
	}

	projects, err := prefills.Load(password)
	if err != nil {
		return writeNativeResponse(w, nativeResponse{Success: false, Error: err.Error()})
	}

	return writeNativeResponse(w, nativeResponse{Success: true, Projects: projects})
}

func readNativeMessage(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	if length == 0 || length > 1024*1024 {
		return nil, fmt.Errorf("invalid message length: %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeNativeResponse(w io.Writer, resp nativeResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	length := uint32(len(data))
	if err := binary.Write(w, binary.LittleEndian, length); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
