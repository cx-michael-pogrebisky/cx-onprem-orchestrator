// Package sast implements the CxSAST on-prem scanner driven through the
// CxConsolePlugin (a Java application launched as `java -jar
// CxConsolePlugin-CLI-*.jar Scan ...`). CxSAST has native per-severity count
// caps, so thresholds are PASS-THROUGH to -SASTCritical/-SASTHigh/-SASTMedium/
// -SASTLow with the limit-1 off-by-one fix (the plugin fails on a strict '>',
// cx is inclusive '>='). Path-glob filters are translated to CxSAST folder/file
// NAME patterns (lossy, with warnings). The plugin requires Java 11+.
package sast

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	execpkg "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exec"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exit"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/filter"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

// MinJavaMajor is the minimum JVM major version required by CxConsolePlugin
// 1.1.x: the bundled org.eclipse.jgit 6.10 is Java 11 bytecode.
const MinJavaMajor = 11

const reportName = "sast"

func init() {
	scanner.Register(model.EngineSAST, func() scanner.Scanner { return &Scanner{} })
}

// Scanner runs CxSAST on-prem.
type Scanner struct{}

func (s *Scanner) Engine() model.Engine { return model.EngineSAST }

func (s *Scanner) Available(_ context.Context, cfg *scanner.Config) error {
	if cfg.Path == "" {
		return fmt.Errorf("CxSAST requires --sast-path (runCxConsole.sh/.cmd, the CxConsolePlugin jar, or the plugin dir)")
	}
	if _, err := locateJar(cfg.Path); err != nil {
		return err
	}
	javaBin := resolveJava(cfg)
	major, err := javaMajor(javaBin)
	if err != nil {
		return fmt.Errorf("CxSAST needs a Java %d+ runtime: %w (set --sast-java)", MinJavaMajor, err)
	}
	if major < MinJavaMajor {
		return fmt.Errorf("CxSAST CxConsolePlugin requires Java >= %d but %s is Java %d (set --sast-java)", MinJavaMajor, javaBin, major)
	}
	if cfg.SASTServer == "" {
		return fmt.Errorf("CxSAST requires --sast-server (or $CXSAST_URL)")
	}
	if !s.hasAuth(cfg) {
		return fmt.Errorf("CxSAST requires auth: set --sast-token-env, or (--sast-user or --sast-user-env) + --sast-password-env, or --sast-sso")
	}
	return nil
}

func (s *Scanner) hasAuth(cfg *scanner.Config) bool {
	if cfg.SASTSSO {
		return true
	}
	if cfg.SASTTokenEnv != "" && os.Getenv(cfg.SASTTokenEnv) != "" {
		return true
	}
	if sastUser(cfg) != "" &&
		cfg.SASTPasswordEnv != "" && os.Getenv(cfg.SASTPasswordEnv) != "" {
		return true
	}
	return false
}

// sastUser returns the CxSAST username: the direct --sast-user value if set, else
// the value of the configured env var. The username is not a secret, so it may be
// supplied on the command line.
func sastUser(cfg *scanner.Config) string {
	if cfg.SASTUser != "" {
		return cfg.SASTUser
	}
	return os.Getenv(cfg.SASTUserEnv)
}

func (s *Scanner) BuildInvocation(cfg *scanner.Config, th threshold.Plan) (*model.Invocation, error) {
	jar, err := locateJar(cfg.Path)
	if err != nil {
		return nil, err
	}
	jarAbs, _ := filepath.Abs(jar)
	pluginDir := filepath.Dir(jarAbs)
	srcAbs, err := filepath.Abs(cfg.Source)
	if err != nil {
		return nil, fmt.Errorf("resolve source: %w", err)
	}
	outAbs, err := filepath.Abs(cfg.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output: %w", err)
	}

	args := []string{"-Xmx1024m", "-jar", jarAbs, "Scan",
		"-CxServer", cfg.SASTServer,
		"-ProjectName", teamProject(cfg.Extra["sastTeam"], cfg.ProjectName),
		"-LocationType", "folder",
		"-LocationPath", srcAbs,
	}
	args = append(args, s.authArgs(cfg)...)

	// Threshold pass-through with the limit-1 off-by-one fix.
	for _, c := range th.Clauses {
		if flag, ok := sastCapFlag(c.Severity); ok {
			args = append(args, flag, strconv.Itoa(threshold.SASTNativeCap(c.Limit)))
		}
	}

	// Lossy glob -> CxSAST name-pattern filters.
	names := filter.ToCxSASTNames(filter.ParseGlob(cfg.FileFilter), filter.ParseGlob(cfg.SASTFilter))
	if len(names.Folders) > 0 {
		args = append(args, "-LocationPathExclude", strings.Join(names.Folders, ","))
	}
	if len(names.Files) > 0 {
		args = append(args, "-LocationFilesExclude", strings.Join(names.Files, ","))
	}

	// Reports. XML is mandatory (wrapper-side count recovery + summary); CxSAST can
	// additionally emit PDF/CSV/RTF. SelectFormats warns on unsupported requests
	// (e.g. json/sarif — the CxConsolePlugin does not produce those).
	formats, fmtWarn := scanner.SelectEngineFormats(model.EngineSAST, cfg.ReportFormats)
	for _, f := range formats {
		if flag, ok := sastReportFlag(f); ok {
			args = append(args, flag, filepath.Join(outAbs, reportName+"."+f))
		}
	}

	args = append(args, cfg.RawArgs...)

	inv := &model.Invocation{
		Engine:    model.EngineSAST,
		Path:      resolveJava(cfg),
		Args:      args,
		WorkDir:   pluginDir, // so the jar's Class-Path lib/ resolves
		OutputDir: outAbs,
		Warnings:  append(append([]string{}, names.Warnings...), fmtWarn...),
	}
	return inv, nil
}

// authArgs builds the credential argv. CxConsolePlugin has no env-based auth, so
// secret values must be passed as flags (visible in the child's process listing
// for the scan duration) — the values are read from the configured env vars.
func (s *Scanner) authArgs(cfg *scanner.Config) []string {
	if cfg.SASTSSO {
		return []string{"-useSSO"}
	}
	if cfg.SASTTokenEnv != "" {
		if tok := os.Getenv(cfg.SASTTokenEnv); tok != "" {
			return []string{"-CxToken", tok}
		}
	}
	return []string{
		"-CxUser", sastUser(cfg),
		"-CxPassword", os.Getenv(cfg.SASTPasswordEnv),
	}
}

func (s *Scanner) Run(ctx context.Context, inv *model.Invocation, opts execpkg.Options) *model.Result {
	r := scanner.RunInvocation(ctx, inv, opts)
	r.Route = model.RoutePassthrough
	r.Warnings = inv.Warnings
	if m, _ := filepath.Glob(filepath.Join(inv.OutputDir, reportName+".*")); len(m) > 0 {
		r.ReportPaths = m
	}
	return r
}

// cxXMLResults is the subset of the CxSAST XML report we parse.
type cxXMLResults struct {
	XMLName xml.Name `xml:"CxXMLResults"`
	Queries []struct {
		Results []struct {
			Severity string `xml:"Severity,attr"`
		} `xml:"Result"`
	} `xml:"Query"`
}

func (s *Scanner) ParseResults(_ context.Context, r *model.Result) error {
	xmlPath := filepath.Join(r.OutputDir, reportName+".xml")
	if _, statErr := os.Stat(xmlPath); statErr != nil {
		return nil // report may be absent on engine failure; gating falls back to exit code
	}
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return fmt.Errorf("read sast report: %w", err)
	}
	var res cxXMLResults
	if err := xml.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("parse sast report: %w", err)
	}
	counts := model.SeverityCount{}
	for _, q := range res.Queries {
		for _, rr := range q.Results {
			if sev, ok := model.ParseSeverity(rr.Severity); ok && sev.IsRealSeverity() {
				counts[sev]++
			}
		}
	}
	r.Counts = counts
	return nil
}

func (s *Scanner) Evaluate(r *model.Result, th threshold.Plan) model.Verdict {
	v := model.Verdict{Engine: model.EngineSAST}
	r.NativeGated = th.HasClauses()
	if r.Err != nil {
		v.Category = model.CatEngineFailure
		v.Message = r.Err.Error()
		return v
	}
	switch exit.InterpretCxSAST(r.ChildExitCode) {
	case model.CatThresholdBreach:
		v.Category = model.CatThresholdBreach
		// Recover accurate breach details from the parsed report when available.
		if len(r.Counts) > 0 && th.HasClauses() {
			v.Breaches = threshold.Enforce(th, r.Counts)
		}
		if len(v.Breaches) == 0 {
			v.Message = fmt.Sprintf("CxSAST reported a threshold breach (exit %d)", r.ChildExitCode)
		}
	case model.CatPrerequisiteMissing:
		v.Category = model.CatPrerequisiteMissing
		v.Message = fmt.Sprintf("CxSAST login failed (exit %d)", r.ChildExitCode)
	case model.CatInterrupted:
		v.Category = model.CatInterrupted
	case model.CatPass:
		v.Pass = true
		v.Category = model.CatPass
	default:
		v.Category = model.CatEngineFailure
		v.Message = fmt.Sprintf("CxSAST error (exit %d)", r.ChildExitCode)
	}
	return v
}

// teamProject builds the CxSAST -ProjectName. CxSAST requires the project to be
// qualified by its full team path (e.g. "CxServer\SP\my-proj"); a bare name is
// rejected with "Invalid project path". The team is normalized to backslash
// separators (so "CxServer/SP" works) and joined to the project name.
func teamProject(team, project string) string {
	team = strings.ReplaceAll(team, "/", "\\")
	team = strings.Trim(team, "\\")
	if team == "" {
		return project
	}
	return team + "\\" + project
}

// sastFormats are the report formats the CxConsolePlugin can emit.
// sastReportFlag maps a report format to its CxConsolePlugin -Report* flag.
func sastReportFlag(format string) (string, bool) {
	switch format {
	case "xml":
		return "-ReportXML", true
	case "pdf":
		return "-ReportPDF", true
	case "csv":
		return "-ReportCSV", true
	case "rtf":
		return "-ReportRTF", true
	default:
		return "", false
	}
}

// sastCapFlag maps a severity to its CxConsolePlugin cap flag.
func sastCapFlag(sev model.Severity) (string, bool) {
	switch sev {
	case model.SevCritical:
		return "-SASTCritical", true
	case model.SevHigh:
		return "-SASTHigh", true
	case model.SevMedium:
		return "-SASTMedium", true
	case model.SevLow:
		return "-SASTLow", true
	default:
		return "", false
	}
}

// locateJar resolves --sast-path (a runCxConsole.sh/.cmd, the jar itself, or the
// plugin directory) to the CxConsolePlugin CLI jar.
func locateJar(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("--sast-path %q not found: %w", path, err)
	}
	dir := path
	if !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(path), ".jar") {
			return path, nil
		}
		dir = filepath.Dir(path) // runCxConsole.sh/.cmd
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "CxConsolePlugin-CLI-*.jar"))
	if len(matches) == 0 {
		return "", fmt.Errorf("no CxConsolePlugin-CLI-*.jar found near %q", path)
	}
	return matches[0], nil
}

// javaBinaryName is the java executable's filename for the given GOOS. On Windows
// it must carry the .exe suffix: an absolute path without it (e.g.
// JAVA_HOME\bin\java) is not launchable, since Windows only appends extensions
// from PATHEXT during a PATH search, not for a fully-qualified path.
func javaBinaryName(goos string) string {
	if goos == "windows" {
		return "java.exe"
	}
	return "java"
}

// resolveJava returns the java binary to use: --sast-java (a JDK home or a java
// path), else $JAVA_HOME/bin/java(.exe), else "java" on PATH.
func resolveJava(cfg *scanner.Config) string {
	javaBin := javaBinaryName(runtime.GOOS)
	if jh := cfg.Extra["sastJava"]; jh != "" {
		if info, err := os.Stat(jh); err == nil && info.IsDir() {
			return filepath.Join(jh, "bin", javaBin)
		}
		return jh
	}
	if jh := os.Getenv("JAVA_HOME"); jh != "" {
		cand := filepath.Join(jh, "bin", javaBin)
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return "java"
}

var javaVerRe = regexp.MustCompile(`version "([0-9]+)(?:\.([0-9]+))?`)

// javaMajor returns the major Java version of a java binary (8 for "1.8.0_x",
// 11 for "11.0.x", etc.).
func javaMajor(javaBin string) (int, error) {
	out, err := exec.Command(javaBin, "-version").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("running %s -version: %w", javaBin, err)
	}
	m := javaVerRe.FindStringSubmatch(string(out))
	if m == nil {
		return 0, fmt.Errorf("could not parse java version from %q", string(out))
	}
	first, _ := strconv.Atoi(m[1])
	if first == 1 && m[2] != "" { // "1.8" style
		return strconv.Atoi(m[2])
	}
	return first, nil
}
