package cli

import (
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/config"
	"github.com/spf13/cobra"
)

// engineTokens are the per-engine flag namespaces registered for mode/path/image/arg.
var engineTokens = []string{"sast", "sca", "kics", "secrets", "containers"}

// boundFlags holds the cobra-bound flag storage and assembles a config.Flags.
type boundFlags struct {
	f             config.Flags
	rawArgs       map[string]*[]string
	mode          map[string]*string
	path          map[string]*string
	image         map[string]*string
	reportFormats map[string]*string
}

// registerRunFlags registers the full run/validate flag surface on cmd and
// returns the binding used to assemble a config.Flags after parsing.
func registerRunFlags(cmd *cobra.Command) *boundFlags {
	b := &boundFlags{
		rawArgs:       map[string]*[]string{},
		mode:          map[string]*string{},
		path:          map[string]*string{},
		image:         map[string]*string{},
		reportFormats: map[string]*string{},
	}
	fs := cmd.Flags()

	// Tier C — selection / context.
	fs.StringVar(&b.f.Scanners, "scanners", "", "Comma-separated engines to run: sast,sca,kics,secrets,containers (or 'all'). Required.")
	fs.StringVarP(&b.f.Source, "source", "s", "", "Path to the code under test (default: CI-detected workspace or '.')")
	fs.StringVar(&b.f.ProjectName, "project-name", "", "Project name for all engines (default: derived from repo/source)")
	fs.StringVar(&b.f.SASTProjectName, "sast-project-name", "", "CxSAST project name override (default: --project-name)")
	fs.StringVar(&b.f.CxProjectName, "cx-project-name", "", "Cx1 (SCA + Container Security) project name override (default: --project-name)")
	fs.StringVar(&b.f.Branch, "branch", "", "Branch name (default: CI-detected)")

	// Tier A — threshold.
	fs.StringVar(&b.f.Threshold, "threshold", "", `cx-style threshold, e.g. "sast-high=10;sca-high=5;iac-security-low=10;secrets-total=1"`)

	// Tier A — filters.
	fs.StringVarP(&b.f.FileFilter, "file-filter", "f", "", "Global glob/Nant source filter (cx-verbatim)")
	fs.StringVarP(&b.f.FileInclude, "file-include", "i", "", "Extra file extensions to include (cx-verbatim)")
	fs.BoolVar(&b.f.UseGitignore, "use-gitignore", false, "Honor the directory .gitignore")
	fs.StringVar(&b.f.SASTFilter, "sast-filter", "", "SAST glob filter (lossy -> CxSAST name patterns)")
	fs.StringVar(&b.f.SCAFilter, "sca-filter", "", "SCA regex filter (verbatim to cx)")
	fs.StringVar(&b.f.IaCFilter, "iac-security-filter", "", "KICS glob filter (-> --exclude-paths)")
	fs.StringVar(&b.f.IaCFilter, "kics-filter", "", "Deprecated alias of --iac-security-filter")
	fs.StringVar(&b.f.ContainersFileFolderFilter, "containers-file-folder-filter", "", "Containers glob filter (verbatim to cx)")
	fs.StringVar(&b.f.ContainersPackageFilter, "containers-package-filter", "", "Containers package regex filter (verbatim to cx)")
	fs.StringVar(&b.f.ContainersImageTagFilter, "containers-image-tag-filter", "", "Containers image-tag wildcard filter (verbatim to cx)")
	fs.StringVar(&b.f.SecretsFilter, "secrets-filter", "", "Secrets name-glob filter (-> 2ms --ignore-pattern)")

	// Tier A — output.
	fs.StringVar(&b.f.ReportFormats, "report-formats", "json,sarif", "Comma-separated report formats")
	fs.StringVar(&b.f.OutputPath, "output-path", "", "Directory for collected reports (default ./cxoo-reports)")
	fs.StringVar(&b.f.OutputName, "output-name", "", "Report file base name (default cxoo)")
	fs.StringVar(&b.f.IgnoreOnExit, "ignore-on-exit", "", "none|results|errors|all (unified)")

	// Orchestration.
	fs.StringVar(&b.f.OnMissing, "on-missing", "", "fail|skip-warn when a requested engine's tool is missing (default fail)")
	fs.StringVar(&b.f.Conflict, "conflict", "", "error|raw-wins|unified-wins for Tier-A/Tier-B collisions (default error)")
	fs.BoolVar(&b.f.Async, "async", false, "Run scans asynchronously (incompatible with --threshold)")
	fs.IntVar(&b.f.Parallel, "parallel", 0, "Run up to N engines concurrently (0 = sequential)")
	fs.BoolVar(&b.f.FailFast, "fail-fast", false, "Stop after the first engine failure")
	fs.DurationVar(&b.f.Timeout, "timeout", 0, "Overall timeout (e.g. 30m); 0 = none")

	// Auth selectors (no secret values).
	// Cx1 auth — mode A: API key (default).
	fs.StringVar(&b.f.CxAPIKeyEnv, "cx-apikey-env", "", "Env var holding the Cx1 API key (default CX1_APIKEY, falling back to CX_APIKEY; auto-derives base/auth/tenant)")
	// Cx1 auth — mode B: OAuth2 client-credentials (requires base-uri + base-auth-uri + tenant).
	fs.StringVar(&b.f.CxClientID, "cx-client-id", "", "Cx1 OAuth2 client ID (or CX_CLIENT_ID); selects client-credentials auth")
	fs.StringVar(&b.f.CxClientSecretEnv, "cx-client-secret-env", "", "Env var holding the Cx1 OAuth2 client secret (default CX_CLIENT_SECRET)")
	fs.StringVar(&b.f.CxClientSecretFile, "cx-client-secret-file", "", "File (0600) holding the Cx1 OAuth2 client secret")
	fs.StringVar(&b.f.CxBaseURI, "cx-base-uri", "", "Cx1 base/system URI, e.g. https://<region>.ast.checkmarx.net (required for client-credentials)")
	fs.StringVar(&b.f.CxBaseAuthURI, "cx-base-auth-uri", "", "Cx1 IAM/auth URI, e.g. https://<region>.iam.checkmarx.net (required for client-credentials)")
	fs.StringVar(&b.f.CxTenant, "cx-tenant", "", "Cx1 tenant (required for client-credentials)")
	fs.StringVar(&b.f.SASTServer, "sast-server", "", "CxSAST server URL -> -CxServer (default $CXSAST_URL)")
	fs.StringVar(&b.f.SASTUser, "sast-user", "", "CxSAST username as a direct value (not a secret; alternative to --sast-user-env)")
	fs.StringVar(&b.f.SASTUserEnv, "sast-user-env", "", "Env var holding the CxSAST user (default CXSAST_USERNAME)")
	fs.StringVar(&b.f.SASTPasswordEnv, "sast-password-env", "", "Env var holding the CxSAST password (default CXSAST_PASSWORD)")
	fs.StringVar(&b.f.SASTTokenEnv, "sast-token-env", "", "Env var holding a CxSAST token (preferred over password)")
	fs.BoolVar(&b.f.SASTSSO, "sast-sso", false, "Use CxSAST Windows SSO (-useSSO)")
	fs.StringVar(&b.f.SASTJava, "sast-java", "", "JDK home or java binary for the CxConsolePlugin (Java 11+; on Windows pass the full path to java.exe)")
	fs.StringVar(&b.f.SASTTeam, "sast-team", "", "CxSAST team / full-path prefix for the project, e.g. \"CxServer/SP\" -> -ProjectName CxServer\\SP\\<project>")

	// SCA resolver / containers inputs.
	fs.StringVar(&b.f.ScaResolverPath, "sca-resolver", "", "Path to the ScaResolver executable (enables SCA Resolver mode)")
	fs.StringArrayVar(&b.f.ScaResolverArgs, "sca-resolver-arg", nil, "Raw arg appended to --sca-resolver-params (repeatable, =-bound)")
	fs.StringVar(&b.f.ContainerImages, "container-images", "", "Comma-separated images for the containers scan")
	fs.StringVar(&b.f.KicsQueries, "kics-queries", "", "Path to the KICS query assets (native mode; else $CXOO_KICS_QUERIES_PATH)")

	// Per-engine mode/path/image/arg.
	for _, tok := range engineTokens {
		mode := new(string)
		path := new(string)
		image := new(string)
		raw := new([]string)
		rf := new(string)
		b.mode[tok] = mode
		b.path[tok] = path
		b.image[tok] = image
		b.rawArgs[tok] = raw
		b.reportFormats[tok] = rf
		fs.StringVar(mode, tok+"-mode", "", "Resolution mode for "+tok+": native|docker")
		fs.StringVar(path, tok+"-path", "", "Binary/JAR/script path for "+tok+" (native)")
		fs.StringVar(image, tok+"-image", "", "Docker image for "+tok+" (docker)")
		fs.StringArrayVar(raw, tok+"-arg", nil, "Raw native arg for "+tok+" (repeatable, =-bound; e.g. --"+tok+"-arg=-Foo=bar)")
		fs.StringVar(rf, tok+"-report-formats", "", "Override --report-formats for "+tok+" only (e.g. --"+tok+"-report-formats=json)")
	}

	return b
}

// toConfigFlags assembles the per-engine maps into the config.Flags value.
func (b *boundFlags) toConfigFlags() config.Flags {
	f := b.f
	f.Mode = map[string]string{}
	f.Path = map[string]string{}
	f.Image = map[string]string{}
	f.RawArgs = map[string][]string{}
	f.ReportFormatsOverride = map[string]string{}
	for _, tok := range engineTokens {
		if v := *b.mode[tok]; v != "" {
			f.Mode[tok] = v
		}
		if v := *b.path[tok]; v != "" {
			f.Path[tok] = v
		}
		if v := *b.image[tok]; v != "" {
			f.Image[tok] = v
		}
		if v := *b.rawArgs[tok]; len(v) > 0 {
			f.RawArgs[tok] = v
		}
		if v := *b.reportFormats[tok]; v != "" {
			f.ReportFormatsOverride[tok] = v
		}
	}
	return f
}
