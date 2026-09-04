package prefill

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/lukaculjak/mak-cli/internal/prefills"
)

func TestIsNativeHostInvocation(t *testing.T) {
	if !IsNativeHostInvocation([]string{"chrome-extension://abcdefghijklmnop/"}) {
		t.Fatal("Chrome extension origin was not recognized")
	}
	if IsNativeHostInvocation([]string{"prefill", "native-host"}) {
		t.Fatal("ordinary CLI arguments were recognized as native host invocation")
	}
}

func TestRunNativeHostUnlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const password = "correct horse battery staple"
	if err := prefills.InitStore(password); err != nil {
		t.Fatal(err)
	}
	if err := prefills.Save(password, []prefills.Project{{Name: "Example"}}); err != nil {
		t.Fatal(err)
	}

	request, err := json.Marshal(nativeRequest{Action: "unlock", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	var input bytes.Buffer
	if err := binary.Write(&input, binary.LittleEndian, uint32(len(request))); err != nil {
		t.Fatal(err)
	}
	_, _ = input.Write(request)

	var output bytes.Buffer
	if err := RunNativeHost(&input, &output); err != nil {
		t.Fatal(err)
	}

	var responseLength uint32
	if err := binary.Read(&output, binary.LittleEndian, &responseLength); err != nil {
		t.Fatal(err)
	}
	responseData := make([]byte, responseLength)
	if _, err := output.Read(responseData); err != nil {
		t.Fatal(err)
	}
	var response nativeResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatal(err)
	}
	if !response.Success || len(response.Projects) != 1 || response.Projects[0].Name != "Example" {
		t.Fatalf("unexpected response: %#v", response)
	}
}
