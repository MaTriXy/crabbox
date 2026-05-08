package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type artifactFile struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url,omitempty"`
}

type artifactBundleMetadata struct {
	CreatedAt string `json:"createdAt"`
	Version   string `json:"crabboxVersion"`
	LeaseID   string `json:"leaseId,omitempty"`
	Slug      string `json:"slug,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Network   string `json:"network,omitempty"`
	TargetOS  string `json:"targetOS,omitempty"`
	RunID     string `json:"runId,omitempty"`
}

type artifactCollectResult struct {
	Directory string                 `json:"directory"`
	Files     []artifactFile         `json:"files"`
	Metadata  artifactBundleMetadata `json:"metadata"`
	Warnings  []artifactWarning      `json:"warnings,omitempty"`
	Error     *artifactCollectError  `json:"error,omitempty"`
}

type artifactCollectError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type artifactWarning struct {
	Problem  string   `json:"problem"`
	Detail   string   `json:"detail,omitempty"`
	Rescue   []string `json:"rescue,omitempty"`
	Fallback string   `json:"fallback,omitempty"`
}

type artifactPublishOptions struct {
	Directory   string
	Storage     string
	Bucket      string
	Prefix      string
	BaseURL     string
	PR          int
	Repo        string
	Template    string
	Summary     string
	SummaryFile string
	Region      string
	Profile     string
	EndpointURL string
	ACL         string
	Presign     bool
	Expires     time.Duration
	DryRun      bool
	NoComment   bool
}

func (a App) artifactsCollect(ctx context.Context, args []string) error {
	defaults := defaultConfig()
	fs := newFlagSet("artifacts collect", a.Stderr)
	provider := fs.String("provider", defaults.Provider, "provider: hetzner, aws, or ssh")
	id := fs.String("id", "", "lease id or slug")
	output := fs.String("output", "", "artifact bundle directory")
	runID := fs.String("run", "", "optional run id whose retained logs should be copied")
	all := fs.Bool("all", false, "collect screenshot, video, GIF, doctor/status, logs, and metadata")
	screenshot := fs.Bool("screenshot", true, "capture desktop screenshot")
	video := fs.Bool("video", false, "record desktop video")
	gif := fs.Bool("gif", false, "create trimmed GIF from recorded video")
	doctor := fs.Bool("doctor", true, "write desktop doctor output")
	webvncStatus := fs.Bool("webvnc-status", true, "write WebVNC portal status when coordinator is configured")
	metadata := fs.Bool("metadata", true, "write metadata.json")
	duration := fs.Duration("duration", 10*time.Second, "video capture duration")
	fps := fs.Float64("fps", 15, "video frames per second")
	gifWidth := fs.Int("gif-width", 640, "trimmed GIF width")
	reclaim := fs.Bool("reclaim", false, "claim this lease for the current repo")
	targetFlags := registerTargetFlags(fs, defaults)
	networkFlags := registerNetworkModeFlag(fs, defaults)
	jsonOut := fs.Bool("json", false, "print machine-readable result")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	setIDFromFirstArg(fs, id)
	if *all {
		*video = true
		*gif = true
	}
	if *gif && !*video {
		return exit(2, "artifacts collect --gif requires --video or --all")
	}
	if *duration <= 0 {
		return exit(2, "artifacts collect --duration must be positive")
	}
	if *fps <= 0 {
		return exit(2, "artifacts collect --fps must be positive")
	}
	if *gifWidth <= 0 {
		return exit(2, "artifacts collect --gif-width must be positive")
	}
	cfg, err := loadLeaseTargetConfig(fs, *provider, targetFlags, networkFlags, leaseTargetConfigOptions{Desktop: true})
	if err != nil {
		return err
	}
	if isBlacksmithProvider(cfg.Provider) {
		return exit(2, "artifacts collect is not supported for provider=%s; Blacksmith owns machine connectivity", cfg.Provider)
	}
	if err := requireLeaseID(*id, "crabbox artifacts collect --id <lease-id-or-slug> [--output <dir>]", cfg); err != nil {
		return err
	}
	server, target, leaseID, err := a.resolveNetworkLeaseTarget(ctx, cfg, *id, false)
	if err != nil {
		return err
	}
	if isStaticProvider(cfg.Provider) && target.TargetOS != targetLinux {
		return exit(2, "desktop artifacts are not collected from static %s hosts because those are existing host machines, not Crabbox-created desktops", target.TargetOS)
	}
	if err := enforceManagedLeaseCapabilities(cfg, server, leaseID); err != nil {
		return err
	}
	if err := a.claimAndTouchLeaseTarget(ctx, cfg, server, leaseID, *reclaim); err != nil {
		return err
	}
	dir := strings.TrimSpace(*output)
	if dir == "" {
		dir = defaultArtifactBundleDir(leaseID, serverSlug(server))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return exit(2, "create artifact directory: %v", err)
	}

	result := artifactCollectResult{
		Directory: dir,
		Metadata: artifactBundleMetadata{
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Version:   version,
			LeaseID:   leaseID,
			Slug:      serverSlug(server),
			Provider:  cfg.Provider,
			Network:   string(cfg.Network),
			TargetOS:  target.TargetOS,
			RunID:     strings.TrimSpace(*runID),
		},
	}
	addFile := func(kind, path string) {
		result.Files = append(result.Files, artifactFile{Kind: kind, Name: filepath.Base(path), Path: path})
	}
	fail := func(err error, warning artifactWarning) error {
		return a.finishArtifactCollectFailure(&result, *jsonOut, err, warning)
	}

	if *metadata {
		path := filepath.Join(dir, "metadata.json")
		if err := writeJSONFile(path, result.Metadata); err != nil {
			return err
		}
		addFile("metadata", path)
	}
	if *screenshot {
		if err := waitForLoopbackVNC(ctx, &target); err != nil {
			return fail(err, artifactWarning{
				Problem: rescueVNCTargetUnreachable,
				Detail:  err.Error(),
				Rescue:  []string{desktopDoctorCommand(rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID})},
			})
		}
		path := filepath.Join(dir, "screenshot.png")
		if err := captureDesktopScreenshot(ctx, target, path); err != nil {
			return fail(err, artifactWarning{
				Problem: classifyDesktopFailure(err.Error()),
				Detail:  err.Error(),
				Rescue:  []string{desktopDoctorCommand(rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID})},
			})
		}
		addFile("screenshot", path)
	}
	if *doctor {
		path := filepath.Join(dir, "doctor.txt")
		out, err := runSSHOutput(ctx, target, desktopDoctorRemoteCommand(target))
		if err != nil {
			doctorErr := exit(5, "desktop doctor failed: %v", err)
			return fail(doctorErr, artifactWarning{
				Problem: classifyDesktopFailure(out),
				Detail:  trimFailureDetail(out),
				Rescue:  []string{desktopDoctorCommand(rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID})},
			})
		}
		if err := os.WriteFile(path, []byte(out+"\n"), 0o644); err != nil {
			return exit(2, "write doctor artifact: %v", err)
		}
		addFile("doctor", path)
	}
	if *webvncStatus {
		if path, ok, err := a.writeArtifactWebVNCStatus(ctx, cfg, target, leaseID, dir, &result.Warnings); err != nil {
			return err
		} else if ok {
			addFile("webvnc-status", path)
		}
	}
	if strings.TrimSpace(*runID) != "" {
		logPath, runPath, err := writeArtifactRunLogs(ctx, strings.TrimSpace(*runID), dir)
		if err != nil {
			return fail(err, artifactWarning{
				Problem: rescueArtifactCaptureFailed,
				Detail:  err.Error(),
				Rescue:  []string{"crabbox logs " + strings.TrimSpace(*runID)},
			})
		}
		addFile("logs", logPath)
		addFile("run", runPath)
	}
	if *video {
		if target.TargetOS != targetLinux {
			err := exit(2, "artifacts collect --video currently requires target=linux with ffmpeg/x11grab")
			return fail(err, artifactWarning{
				Problem: rescueArtifactCaptureFailed,
				Detail:  err.Error(),
				Rescue:  []string{desktopDoctorCommand(rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID})},
			})
		}
		path := filepath.Join(dir, "screen.mp4")
		if err := captureDesktopVideo(ctx, target, path, *duration, *fps); err != nil {
			return fail(err, artifactWarning{
				Problem: classifyDesktopFailure(err.Error()),
				Detail:  err.Error(),
				Rescue:  []string{desktopDoctorCommand(rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID})},
			})
		}
		addFile("video", path)
		if *gif {
			gifPath := filepath.Join(dir, "screen.trimmed.gif")
			trimmedPath := filepath.Join(dir, "screen.trimmed.mp4")
			preview, err := createMediaPreview(ctx, mediaPreviewOptions{
				Input:              path,
				Output:             gifPath,
				TrimmedVideoOutput: trimmedPath,
				Width:              *gifWidth,
				FPS:                4,
				TrimStatic:         true,
				TrimPadding:        750 * time.Millisecond,
				FreezeDuration:     500 * time.Millisecond,
				FreezeNoise:        "-50dB",
				MinDuration:        1500 * time.Millisecond,
			})
			if err != nil {
				return fail(err, artifactWarning{
					Problem: rescueArtifactCaptureFailed,
					Detail:  err.Error(),
				})
			}
			addFile("gif", preview.Output)
			if preview.TrimmedVideoOutput != "" {
				addFile("trimmed-video", preview.TrimmedVideoOutput)
			}
		}
	}
	sortArtifactFiles(result.Files)
	if result.Files == nil {
		result.Files = []artifactFile{}
	}
	if *jsonOut {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	for _, warning := range result.Warnings {
		printArtifactWarning(a.Stdout, warning)
	}
	fmt.Fprintf(a.Stdout, "artifacts: %s\n", dir)
	for _, file := range result.Files {
		fmt.Fprintf(a.Stdout, "%s: %s\n", file.Kind, file.Path)
	}
	fmt.Fprintf(a.Stdout, "publish: crabbox artifacts publish --dir %s --pr <n>\n", strings.Join(readableShellWords([]string{dir}), " "))
	return nil
}

func (a App) finishArtifactCollectFailure(result *artifactCollectResult, jsonOut bool, err error, warning artifactWarning) error {
	if result == nil {
		return err
	}
	sortArtifactFiles(result.Files)
	if result.Files == nil {
		result.Files = []artifactFile{}
	}
	if strings.TrimSpace(warning.Problem) != "" {
		result.Warnings = append(result.Warnings, normalizeArtifactWarning(warning))
	}
	result.Error = &artifactCollectError{
		Code:    artifactErrorCode(result.Warnings),
		Message: strings.TrimSpace(err.Error()),
	}
	if jsonOut {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		if encodeErr := enc.Encode(result); encodeErr != nil {
			return encodeErr
		}
		return err
	}
	for _, warning := range result.Warnings {
		printArtifactWarning(a.Stdout, warning)
	}
	return err
}

func (a App) artifactsVideo(ctx context.Context, args []string) error {
	target, cfg, leaseID, err := a.desktopCommandTarget(ctx, "artifacts video", args, false)
	if err != nil {
		return err
	}
	output, _ := stringFlagValue(args, "output")
	if strings.TrimSpace(output) == "" {
		output = "crabbox-" + normalizeLeaseSlug(leaseID) + "-screen.mp4"
	}
	duration := durationFlagValue(args, "duration", 10*time.Second)
	fps := floatFlagValue(args, "fps", 15)
	if duration <= 0 {
		return exit(2, "artifacts video --duration must be positive")
	}
	if fps <= 0 {
		return exit(2, "artifacts video --fps must be positive")
	}
	if target.TargetOS != targetLinux {
		return exit(2, "artifacts video currently requires target=linux with ffmpeg/x11grab")
	}
	if err := captureDesktopVideo(ctx, target, output, duration, fps); err != nil {
		printRescue(a.Stdout, classifyDesktopFailure(err.Error()), err.Error(), desktopDoctorCommand(rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID}))
		return err
	}
	fmt.Fprintf(a.Stdout, "video: %s\n", output)
	return nil
}

func (a App) artifactsGif(ctx context.Context, args []string) error {
	return a.mediaPreview(ctx, args)
}

func (a App) artifactsTemplate(ctx context.Context, args []string) error {
	_ = ctx
	initialKind := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		initialKind = args[0]
		args = args[1:]
	}
	fs := newFlagSet("artifacts template", a.Stderr)
	kind := fs.String("kind", initialKind, "template kind: openclaw or mantis")
	before := fs.String("before", "", "before screenshot/GIF URL or path")
	after := fs.String("after", "", "after screenshot/GIF URL or path")
	summary := fs.String("summary", "", "summary text")
	summaryFile := fs.String("summary-file", "", "summary markdown file")
	output := fs.String("output", "", "output markdown path; stdout when omitted")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	text, err := summaryText(*summary, *summaryFile)
	if err != nil {
		return err
	}
	body := artifactTemplateMarkdown(*kind, text, *before, *after, nil)
	if strings.TrimSpace(*output) == "" {
		fmt.Fprint(a.Stdout, body)
		return nil
	}
	if err := os.WriteFile(*output, []byte(body), 0o644); err != nil {
		return exit(2, "write template: %v", err)
	}
	fmt.Fprintf(a.Stdout, "template: %s\n", *output)
	return nil
}

func (a App) artifactsPublish(ctx context.Context, args []string) error {
	opts, err := parseArtifactPublishOptions(args, a.Stderr)
	if err != nil {
		return err
	}
	var coord *CoordinatorClient
	if opts.Storage == "auto" || opts.Storage == "broker" {
		cfg, cfgErr := loadConfig()
		if cfgErr != nil {
			return cfgErr
		}
		var useCoordinator bool
		coord, useCoordinator, err = newCoordinatorClient(cfg)
		if err != nil {
			return err
		}
		if opts.Storage == "auto" {
			if useCoordinator && coord != nil && coord.Token != "" {
				opts.Storage = "broker"
			} else {
				opts.Storage = "local"
			}
		}
	}
	ensureArtifactPublishPrefix(&opts)
	files, err := listArtifactBundleFiles(opts.Directory)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return exit(2, "artifact directory has no files: %s", opts.Directory)
	}
	summary, err := summaryText(opts.Summary, opts.SummaryFile)
	if err != nil {
		return err
	}
	var published []artifactFile
	if opts.Storage == "broker" {
		published, err = publishArtifactFilesBroker(ctx, coord, opts, files)
	} else {
		published, err = publishArtifactFiles(ctx, opts, files)
	}
	if err != nil {
		return err
	}
	body := artifactTemplateMarkdown(opts.Template, summary, "", "", published)
	bodyPath := filepath.Join(opts.Directory, "published-artifacts.md")
	if err := os.WriteFile(bodyPath, []byte(body), 0o644); err != nil {
		return exit(2, "write publish markdown: %v", err)
	}
	if opts.PR > 0 && !opts.NoComment {
		if opts.Storage == "local" && opts.BaseURL == "" {
			return exit(2, "artifacts publish --pr needs brokered publishing, --storage s3|r2|cloudflare, or --base-url for already-hosted local assets")
		}
		if opts.DryRun {
			fmt.Fprintf(a.Stdout, "dry-run comment: gh issue comment %d --body-file %s\n", opts.PR, bodyPath)
		} else if err := postGitHubPRComment(ctx, opts.PR, opts.Repo, bodyPath); err != nil {
			return err
		}
	}
	for _, file := range published {
		if file.URL != "" {
			fmt.Fprintf(a.Stdout, "%s: %s\n", file.Kind, file.URL)
		} else {
			fmt.Fprintf(a.Stdout, "%s: %s\n", file.Kind, file.Path)
		}
	}
	fmt.Fprintf(a.Stdout, "markdown: %s\n", bodyPath)
	return nil
}

func defaultArtifactBundleDir(leaseID, slug string) string {
	name := strings.TrimSpace(slug)
	if name == "" {
		name = leaseID
	}
	if name == "" {
		name = time.Now().UTC().Format("20060102-150405")
	}
	return filepath.Join("artifacts", normalizeLeaseSlug(name))
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return exit(2, "encode %s: %v", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return exit(2, "write %s: %v", path, err)
	}
	return nil
}

func (a App) writeArtifactWebVNCStatus(ctx context.Context, cfg Config, target SSHTarget, leaseID, dir string, warnings *[]artifactWarning) (string, bool, error) {
	if isStaticProvider(cfg.Provider) || isBlacksmithProvider(cfg.Provider) {
		return "", false, nil
	}
	coord, useCoordinator, err := newTargetCoordinatorClient(cfg)
	if err != nil || !useCoordinator || coord == nil || coord.Token == "" {
		return "", false, nil
	}
	status, err := coord.WebVNCStatus(ctx, leaseID)
	path := filepath.Join(dir, "webvnc-status.json")
	payload := map[string]any{"leaseId": leaseID, "target": target.TargetOS}
	if err != nil {
		payload["error"] = err.Error()
		rescueCtx := rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID}
		appendArtifactWarning(warnings, rescueVNCBridgeDisconnected, err.Error(), "", webVNCStatusRescueCommand(rescueCtx), webVNCResetRescueCommand(rescueCtx))
	} else {
		payload["status"] = status
		rescueCtx := rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID}
		if !status.BridgeConnected {
			appendArtifactWarning(warnings, rescueVNCBridgeNotRunning, "portal has no active WebVNC bridge for this lease", "", webVNCDaemonStartRescueCommand(rescueCtx), webVNCResetRescueCommand(rescueCtx))
		} else if webVNCObserverSlotsExhausted(status) {
			appendArtifactWarning(warnings, rescueVNCObserverSlotsFull, "all WebVNC observer slots are in use or stale", "", webVNCDaemonStartRescueCommand(rescueCtx), webVNCResetRescueCommand(rescueCtx))
		}
	}
	if err := writeJSONFile(path, payload); err != nil {
		return "", false, err
	}
	return path, true, nil
}

func appendArtifactWarning(warnings *[]artifactWarning, problem, detail, fallback string, rescue ...string) {
	if warnings == nil {
		return
	}
	clean := normalizeArtifactWarning(artifactWarning{Problem: problem, Detail: detail, Fallback: fallback, Rescue: rescue})
	if clean.Problem != "" {
		*warnings = append(*warnings, clean)
	}
}

func normalizeArtifactWarning(warning artifactWarning) artifactWarning {
	clean := artifactWarning{
		Problem:  strings.TrimSpace(warning.Problem),
		Detail:   strings.TrimSpace(warning.Detail),
		Fallback: strings.TrimSpace(warning.Fallback),
	}
	for _, command := range warning.Rescue {
		if strings.TrimSpace(command) != "" {
			clean.Rescue = append(clean.Rescue, strings.TrimSpace(command))
		}
	}
	return clean
}

func artifactErrorCode(warnings []artifactWarning) string {
	if len(warnings) == 0 || strings.TrimSpace(warnings[len(warnings)-1].Problem) == "" {
		return "artifact_collect_failed"
	}
	return normalizeLeaseSlug(warnings[len(warnings)-1].Problem)
}

func printArtifactWarning(w io.Writer, warning artifactWarning) {
	printRescueWithFallback(w, warning.Problem, warning.Detail, warning.Fallback, warning.Rescue...)
}

func writeArtifactRunLogs(ctx context.Context, runID, dir string) (string, string, error) {
	coord, err := configuredCoordinator()
	if err != nil {
		return "", "", err
	}
	logText, err := coord.RunLogs(ctx, runID)
	if err != nil {
		return "", "", err
	}
	run, err := coord.Run(ctx, runID)
	if err != nil {
		return "", "", err
	}
	logPath := filepath.Join(dir, "logs.txt")
	runPath := filepath.Join(dir, "run.json")
	if err := os.WriteFile(logPath, []byte(logText), 0o644); err != nil {
		return "", "", exit(2, "write logs artifact: %v", err)
	}
	if err := writeJSONFile(runPath, run); err != nil {
		return "", "", err
	}
	return logPath, runPath, nil
}

func captureDesktopVideo(ctx context.Context, target SSHTarget, outputPath string, duration time.Duration, fps float64) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil && filepath.Dir(outputPath) != "." {
		return exit(2, "create video directory: %v", err)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return exit(2, "create video %s: %v", outputPath, err)
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(outputPath)
		}
	}()
	if err := runSSHToWriter(ctx, target, desktopVideoRemoteCommand(duration, fps), file); err != nil {
		return exit(5, "capture video: %v", err)
	}
	ok = true
	return nil
}

func desktopVideoRemoteCommand(duration time.Duration, fps float64) string {
	seconds := strconv.FormatFloat(duration.Seconds(), 'f', 3, 64)
	frameRate := strconv.FormatFloat(fps, 'f', 3, 64)
	return fmt.Sprintf(`set -eu
export DISPLAY="${DISPLAY:-:99}"
if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "missing ffmpeg; warm a new --desktop lease or install ffmpeg" >&2
  exit 127
fi
if command -v xdpyinfo >/dev/null 2>&1; then
  size="$(xdpyinfo | awk '/dimensions:/{print $2; exit}')"
else
  size=""
fi
if [ -z "$size" ]; then size="1920x1080"; fi
ffmpeg -hide_banner -loglevel error -y -f x11grab -video_size "$size" -framerate %s -i "$DISPLAY" -t %s -pix_fmt yuv420p -an -movflags frag_keyframe+empty_moov -f mp4 -
`, frameRate, seconds)
}
