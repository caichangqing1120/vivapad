package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vivaui/viva/update"
)

func TestSignedUpdatePrefersPatchAndFallsBack(t *testing.T) {
	for _, mode := range []string{"delta", "base mismatch", "no patch", "tampered signature", "corrupt full"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newFixture(t, mode)
			candidate, err := fixture.engine.Check(t.Context())
			if mode == "tampered signature" {
				if !errors.Is(err, update.ErrSignature) || candidate != nil || fixture.downloads.Load() != 0 {
					t.Fatalf("tampered feed: %v %v", candidate, err)
				}
				return
			}
			if err != nil || candidate == nil || candidate.Version() != "1.1.0" {
				t.Fatalf("check: %v %v", candidate, err)
			}
			if candidate.FullSize() != int64(len(fixture.full)) || (candidate.PatchFrom() != "") != (mode != "no patch" && mode != "corrupt full") || candidate.PatchFrom() != "" && candidate.PatchSize() != int64(len(fixture.patch)) {
				t.Fatalf("wrong asset metadata: %+v", candidate)
			}
			prepared, err := fixture.engine.Prepare(t.Context(), candidate, nil)
			if mode == "corrupt full" {
				if !errors.Is(err, update.ErrChecksum) || prepared != nil {
					t.Fatalf("accepted a corrupt full package: %v %v", prepared, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(prepared.Close)
			if prepared.IsDelta() != (mode == "delta") || prepared.FellBack() != (mode == "base mismatch") {
				t.Fatalf("unexpected result: %+v", prepared)
			}
			if mode == "delta" {
				data, err := os.ReadFile(filepath.Join(prepared.options.Payload, "fixture"))
				if err != nil || string(data) != "base payload" || fixture.downloads.Load() != 0 {
					t.Fatalf("delta result: %q %v, full requests: %d", data, err, fixture.downloads.Load())
				}
			} else {
				path := prepared.options.Artifact
				if runtime.GOOS == "windows" {
					path = prepared.options.Payload
				}
				if fixture.downloads.Load() != 1 || !strings.HasPrefix(path, fixture.engine.dir+string(os.PathSeparator)) {
					t.Fatalf("full download: %+v, requests: %d", prepared, fixture.downloads.Load())
				}
			}
		})
	}
}

func TestCancelUpdateResumesAndNeverFallsBackOnCancel(t *testing.T) {
	for _, mode := range []string{"cancel full", "cancel delta"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newFixture(t, mode)
			candidate, err := fixture.engine.Check(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			prepared, err := fixture.engine.Prepare(ctx, candidate, func(progress Progress) {
				if progress.Done > 0 {
					cancel()
				}
			})
			if prepared != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled download = %v, %v", prepared, err)
			}
			if mode == "cancel delta" && fixture.downloads.Load() != 0 {
				t.Fatal("cancellation downloaded the full package")
			}
			prepared, err = fixture.engine.Prepare(t.Context(), candidate, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Close()
			if !fixture.resumed.Load() {
				t.Fatal("retry did not use Range")
			}
		})
	}
}

func TestInstallManagedExternallyDoesNotQuit(t *testing.T) {
	fixture := newFixture(t, "no patch")
	candidate, err := fixture.engine.Check(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := fixture.engine.Prepare(t.Context(), candidate, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	fixture.engine.install = func(update.InstallOptions, func()) error { return update.ErrManagedExternally }
	if err := fixture.engine.Install(prepared, func() { t.Fatal("called quit") }); !errors.Is(err, update.ErrManagedExternally) || !strings.Contains(err.Error(), "system package manager") {
		t.Fatalf("install result: %v", err)
	}
}

func TestStageWindowsPortableZIP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.zip")
	for _, name := range []string{"../vivapad.exe", "vivapad.exe"} {
		if err := os.WriteFile(path, portableZIP(t, name, []byte("new executable")), 0600); err != nil {
			t.Fatal(err)
		}
		staging, err := stageWindowsZIP(t.Context(), path, dir)
		if name != "vivapad.exe" {
			if err == nil || staging != "" {
				t.Fatalf("unexpected staged archive: %s %v", staging, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(staging, name))
		if string(data) != "new executable" || err != nil {
			t.Fatalf("executable = %q, %v", data, err)
		}
	}
}

type updateFixture struct {
	engine        *Engine
	full, patch   []byte
	downloads     atomic.Int32
	deltaRequests atomic.Int32
	resumed       atomic.Bool
}

func newFixture(t *testing.T, mode string) *updateFixture {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &updateFixture{full: bytes.Repeat([]byte("signed archive bytes"), 10000)}
	if runtime.GOOS == "windows" {
		fixture.full = portableZIP(t, "vivapad.exe", fixture.full)
	}
	fixture.patch = patchFixture(t)
	var body, sig []byte
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/latest.json":
			_, _ = writer.Write(body)
		case "/latest.json.sig":
			_, _ = writer.Write(sig)
		case "/full", "/delta":
			data, calls, cancelMode := fixture.full, &fixture.downloads, "cancel full"
			if request.URL.Path == "/delta" {
				data, calls, cancelMode = fixture.patch, &fixture.deltaRequests, "cancel delta"
			}
			number := calls.Add(1)
			if mode == cancelMode && number == 1 {
				writer.Header().Set("Content-Length", fmt.Sprint(len(data)))
				_, _ = writer.Write(data[:len(data)/2])
				writer.(http.Flusher).Flush()
				<-request.Context().Done()
				return
			}
			if request.Header.Get("Range") != "" {
				fixture.resumed.Store(true)
			}
			if mode == "corrupt full" {
				data = bytes.Repeat([]byte{'x'}, len(data))
			}
			http.ServeContent(writer, request, "", time.Time{}, bytes.NewReader(data))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	format := update.FormatZIP
	if runtime.GOOS == "linux" {
		format = update.FormatTarGz
	}
	asset := update.Asset{OS: runtime.GOOS, Arch: runtime.GOARCH, Format: format, URL: server.URL + "/full", Size: int64(len(fixture.full)), SHA256: digest(fixture.full)}
	if mode != "no patch" && mode != "cancel full" && mode != "corrupt full" {
		asset.Patches = []update.Patch{{FromVersion: "1.0.0", URL: server.URL + "/delta", Size: int64(len(fixture.patch)), SHA256: digest(fixture.patch)}}
	}
	assets := []update.Asset{asset}
	if runtime.GOOS == "windows" {
		installer := asset
		installer.Format, installer.Patches = update.FormatNSIS, nil
		assets = append([]update.Asset{installer}, assets...)
	}
	body, err = json.Marshal(update.Feed{Schema: update.SchemaVersion, App: AppID, Channel: "stable", Latest: update.Release{Version: "1.1.0", Assets: assets}})
	if err != nil {
		t.Fatal(err)
	}
	sig = []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, body)))
	if mode == "tampered signature" {
		sig[0] ^= 1
	}
	fixture.engine, err = New(t.TempDir(), "1.0.0", "stable", server.URL+"/latest.json", base64.StdEncoding.EncodeToString(publicKey), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if mode != "base mismatch" {
		if err := os.WriteFile(filepath.Join(root, "fixture"), []byte("base payload"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	fixture.engine.installed = func() (string, string, error) { return root, payloadKind(), nil }
	return fixture
}

func patchFixture(t *testing.T) []byte {
	t.Helper()
	manifest := update.PatchManifest{Schema: update.PatchSchema, App: AppID, FromVersion: "1.0.0", ToVersion: "1.1.0", OS: runtime.GOOS, Arch: runtime.GOARCH, Payload: payloadKind(), Files: []update.PatchFile{{Path: "fixture", Op: "same", Mode: 0755, Size: 12, SHA256: digest([]byte("base payload"))}}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: "patch.json", Mode: 0600, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func portableZIP(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	entry, err := archive.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
