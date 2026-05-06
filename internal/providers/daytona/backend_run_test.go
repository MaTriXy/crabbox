package daytona

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apidaytona "github.com/daytonaio/daytona/libs/api-client-go"
)

func TestCreateDaytonaSyncArchiveWritesTempFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, err := createDaytonaSyncArchive(t.Context(), Repo{Root: root}, SyncManifest{Files: []string{"hello.txt"}, Bytes: 5}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	info, err := archive.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("archive temp file is empty")
	}
	if _, err := archive.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == "hello.txt" {
			return
		}
	}
	t.Fatal("archive missing hello.txt")
}

func TestDaytonaToolboxUploadURL(t *testing.T) {
	sandbox := &apidaytona.Sandbox{}
	sandbox.SetToolboxProxyUrl("https://proxy.example/base/")
	got, err := daytonaToolboxUploadURL(sandbox, "sbx-123", "/tmp/crabbox archive.tgz")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://proxy.example/base/sbx-123/files/upload?path=%2Ftmp%2Fcrabbox+archive.tgz"
	if got != want {
		t.Fatalf("url=%q, want %q", got, want)
	}
}

func TestUploadDaytonaFileStreamDoesNotPrebuffer(t *testing.T) {
	sourceReader, sourceWriter := io.Pipe()
	requestStarted := make(chan struct{})
	bodyRead := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s, want POST", r.Method)
		}
		if r.URL.Path != "/sbx-123/files/upload" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("path") != "/tmp/archive.tgz" {
			t.Errorf("query path=%q", r.URL.Query().Get("path"))
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization=%q", r.Header.Get("Authorization"))
		}
		close(requestStarted)
		reader, err := r.MultipartReader()
		if err != nil {
			t.Errorf("multipart reader: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Errorf("next part: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if part.FormName() != "file" {
			t.Errorf("form name=%q", part.FormName())
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Errorf("read part: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		bodyRead <- data
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- uploadDaytonaFileStream(t.Context(), srv.Client(), srv.URL+"/sbx-123/files/upload?path=%2Ftmp%2Farchive.tgz", map[string]string{
			"Authorization": "Bearer token",
		}, sourceReader, "archive.tgz")
	}()
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("upload did not start until the source reader completed")
	}
	if _, err := sourceWriter.Write([]byte("hello archive")); err != nil {
		t.Fatal(err)
	}
	if err := sourceWriter.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("upload did not finish")
	}
	select {
	case got := <-bodyRead:
		if string(got) != "hello archive" {
			t.Fatalf("body=%q", got)
		}
	default:
		t.Fatal("server did not read body")
	}
}

func TestDaytonaAuthRequiresOrganizationForJWT(t *testing.T) {
	cfg := baseConfig()
	cfg.Provider = daytonaProvider
	cfg.Daytona.APIKey = ""
	cfg.Daytona.JWTToken = "jwt"
	cfg.Daytona.OrganizationID = ""
	_, err := newDaytonaClient(cfg, Runtime{})
	if err == nil || !strings.Contains(err.Error(), "DAYTONA_ORGANIZATION_ID") {
		t.Fatalf("err=%v, want organization requirement", err)
	}
}

func TestDaytonaSSHTargetUsesReturnedSSHCommand(t *testing.T) {
	cfg := baseConfig()
	cfg.Daytona.SSHGatewayHost = "fallback.example"
	target, err := daytonaSSHTargetFromAccess(cfg, daytonaSSHAccess{
		Token:   "tok_live_secret",
		Command: "ssh -p 2222 tok_live_secret@region-ssh.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.User != "tok_live_secret" || target.Host != "region-ssh.example.com" || target.Port != "2222" {
		t.Fatalf("target=%#v", target)
	}
	if target.Key != "" || !target.AuthSecret || target.NetworkKind != NetworkPublic {
		t.Fatalf("target auth/network=%#v", target)
	}
}

func TestDaytonaSSHTargetFallsBackWhenCommandMissing(t *testing.T) {
	cfg := baseConfig()
	cfg.Daytona.SSHGatewayHost = "fallback.example"
	target, err := daytonaSSHTargetFromAccess(cfg, daytonaSSHAccess{Token: "tok_live_secret"})
	if err != nil {
		t.Fatal(err)
	}
	if target.User != "tok_live_secret" || target.Host != "fallback.example" || target.Port != "22" {
		t.Fatalf("target=%#v", target)
	}
}

func TestDaytonaBackendIsHybridSDKRunAndSSHAccess(t *testing.T) {
	backend := NewDaytonaLeaseBackend(ProviderSpec{Name: daytonaProvider}, baseConfig(), Runtime{})
	if _, ok := backend.(DelegatedRunBackend); !ok {
		t.Fatal("daytona should use delegated SDK run path")
	}
	if _, ok := backend.(SSHLeaseBackend); !ok {
		t.Fatal("daytona should still expose explicit SSH access")
	}
}

func TestDaytonaCommandString(t *testing.T) {
	if got := daytonaCommandString([]string{"go", "test", "./..."}, false); got != "'go' 'test' './...'" {
		t.Fatalf("command=%q", got)
	}
	if got := daytonaCommandString([]string{"FOO=bar", "go", "test"}, false); !strings.Contains(got, "FOO=") || !strings.Contains(got, "go") {
		t.Fatalf("shell command=%q", got)
	}
	if got := daytonaCommandString([]string{"echo hello && pwd"}, true); got != "echo hello && pwd" {
		t.Fatalf("shell mode=%q", got)
	}
}
