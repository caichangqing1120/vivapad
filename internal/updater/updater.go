package updater

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vivaui/viva/update"
)

const AppID = "dev.vivapad.app"

var ErrNotConfigured = errors.New("signed updates are not configured for this build")

type Engine struct {
	client    *update.Client
	dir       string
	current   string
	installed func() (string, string, error)
	install   func(update.InstallOptions, func()) error
}

type Candidate struct {
	engine  *Engine
	version string
	full    update.Asset
	delta   update.Asset
	patch   update.Patch
}

func (candidate *Candidate) Version() string   { return candidate.version }
func (candidate *Candidate) FullSize() int64   { return candidate.full.Size }
func (candidate *Candidate) PatchFrom() string { return candidate.patch.FromVersion }
func (candidate *Candidate) PatchSize() int64  { return candidate.patch.Size }

type Progress struct {
	Done, Total int64
	Applying    bool
	Full        bool
}

type Prepared struct {
	engine   *Engine
	options  update.InstallOptions
	delta    bool
	fellBack bool
}

func (prepared *Prepared) IsDelta() bool  { return prepared.delta }
func (prepared *Prepared) FellBack() bool { return prepared.fellBack }

func (prepared *Prepared) Close() {
	if prepared != nil && prepared.options.Payload != "" {
		_ = os.RemoveAll(prepared.options.Payload)
		prepared.options.Payload = ""
	}
}

func New(configDir, current, channel, feedURL, publicKey string, httpClient *http.Client) (*Engine, error) {
	if configDir == "" || channel == "" || feedURL == "" || publicKey == "" {
		return nil, ErrNotConfigured
	}
	key, err := update.ParsePublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	client, err := update.New(update.Config{FeedURL: feedURL, PublicKeys: []ed25519.PublicKey{key}, App: AppID, Channel: channel, Current: current, HTTP: httpClient})
	if err != nil {
		return nil, err
	}
	return &Engine{client: client, current: current, dir: filepath.Join(configDir, "updates"), installed: installedPayload, install: update.Install}, nil
}

func (engine *Engine) Check(ctx context.Context) (*Candidate, error) {
	feed, err := engine.client.Check(ctx)
	if err != nil || feed == nil {
		return nil, err
	}
	asset, err := feed.Asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	candidate := &Candidate{engine: engine, version: feed.Latest.Version, full: asset}
	if !supportsPayload(asset.Format) && !(runtime.GOOS == "windows" && asset.Format == update.FormatNSIS) {
		return nil, update.ErrNoAsset
	}
	for _, delta := range feed.Latest.Assets {
		if delta.OS != runtime.GOOS || delta.Arch != runtime.GOARCH || !supportsPayload(delta.Format) {
			continue
		}
		if runtime.GOOS == "windows" {
			candidate.full = delta
		}
		if patch, ok := delta.PatchFrom(engine.current); ok && (candidate.patch.Size == 0 || patch.Size < candidate.patch.Size) {
			candidate.delta, candidate.patch = delta, patch
		}
	}
	return candidate, nil
}

func supportsPayload(format string) bool {
	if runtime.GOOS == "linux" {
		return format == update.FormatTarGz
	}
	return format == update.FormatZIP
}

func (engine *Engine) Prepare(ctx context.Context, candidate *Candidate, progress func(Progress)) (*Prepared, error) {
	if candidate == nil || candidate.engine != engine {
		return nil, errors.New("check for a signed update before downloading")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(engine.dir, 0700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(engine.dir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("update storage is not a regular directory")
	}
	if err := os.Chmod(engine.dir, 0700); err != nil {
		return nil, err
	}
	prepared := &Prepared{engine: engine, options: update.InstallOptions{Version: candidate.version, Asset: candidate.full}}
	if runtime.GOOS == "darwin" {
		prepared.options.App = AppID
	} else if runtime.GOOS == "linux" {
		prepared.options.App = "vivapad"
	}
	report := func(value Progress) {
		if progress != nil {
			progress(value)
		}
	}
	if candidate.patch.FromVersion != "" {
		path, err := engine.client.DownloadPatch(ctx, candidate.delta, candidate.patch, engine.dir, func(done, total int64) {
			report(Progress{Done: done, Total: total})
		})
		if err == nil && ctx.Err() == nil {
			report(Progress{Done: candidate.patch.Size, Total: candidate.patch.Size, Applying: true})
			prepared.options.Payload, err = engine.applyPatch(ctx, path, candidate.version)
			if err == nil {
				prepared.options.Asset = candidate.delta
				prepared.delta = true
				return prepared, nil
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		prepared.fellBack = true
	}
	report(Progress{Total: candidate.full.Size, Full: true})
	path, err := engine.client.Download(ctx, candidate.full, engine.dir, func(done, total int64) {
		report(Progress{Done: done, Total: total, Full: true})
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" && candidate.full.Format == update.FormatZIP {
		prepared.options.Payload, err = stageWindowsZIP(ctx, path, engine.dir)
		if err != nil {
			return nil, err
		}
	} else {
		prepared.options.Artifact = path
	}
	return prepared, nil
}

func (engine *Engine) Install(prepared *Prepared, quit func()) error {
	if prepared == nil || prepared.engine != engine || quit == nil || (prepared.options.Payload == "" && prepared.options.Artifact == "") {
		return errors.New("prepare a signed update and approve closing before installing")
	}
	err := engine.install(prepared.options, quit)
	if errors.Is(err, update.ErrManagedExternally) {
		return fmt.Errorf("Use your system package manager to update VivaPad: %w", err)
	}
	return err
}

func payloadKind() string {
	switch runtime.GOOS {
	case "darwin":
		return "app"
	case "windows":
		return "dir"
	default:
		return "file"
	}
}

func installedPayload() (string, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	kind := payloadKind()
	if runtime.GOOS != "windows" {
		executable, err = filepath.EvalSymlinks(executable)
		if err != nil {
			return "", "", err
		}
	}
	root := filepath.Dir(executable)
	if kind == "app" {
		contents := filepath.Dir(root)
		if filepath.Base(root) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(filepath.Dir(contents), ".app") {
			return "", "", update.ErrNotWritable
		}
		root = filepath.Dir(contents)
	}
	return root, kind, nil
}

func (engine *Engine) applyPatch(ctx context.Context, path, version string) (string, error) {
	type result struct {
		path string
		err  error
	}
	finished := make(chan result)
	go func() {
		staging, err := engine.stagePatch(ctx, path, version)
		select {
		case finished <- result{staging, err}:
		case <-ctx.Done():
			if staging != "" {
				_ = os.RemoveAll(staging)
			}
		}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case outcome := <-finished:
		if err := ctx.Err(); err != nil {
			if outcome.path != "" {
				_ = os.RemoveAll(outcome.path)
			}
			return "", err
		}
		return outcome.path, outcome.err
	}
}

func (engine *Engine) stagePatch(ctx context.Context, path, version string) (string, error) {
	patch, err := update.ReadPatch(path)
	if err != nil {
		return "", err
	}
	defer patch.Close()
	manifest := patch.Manifest()
	root, kind, err := engine.installed()
	if err != nil {
		return "", err
	}
	if manifest.App != AppID || manifest.FromVersion != engine.current || manifest.ToVersion != version || manifest.OS != runtime.GOOS || manifest.Arch != runtime.GOARCH || manifest.Payload != kind {
		return "", fmt.Errorf("%w: patch does not match this installation", update.ErrManifest)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(engine.dir, "staging-")
	if err != nil {
		return "", err
	}
	// ApplyPatch is a convenience wrapper that can only stage in the system temp directory.
	if err := patch.Apply(root, staging); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	return staging, nil
}
