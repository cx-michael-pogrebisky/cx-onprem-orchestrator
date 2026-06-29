package cli

import (
	"encoding/json"
	"strings"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SchemaFlag describes one CLI flag for external tooling (the config builder).
type SchemaFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`              // string|bool|int|duration|stringArray
	Default   string `json:"default,omitempty"` // pflag DefValue
	Usage     string `json:"usage"`
	Group     string `json:"group"`            // selection|threshold|filters|output|orchestration|auth-cx1|auth-sast|tool|passthrough
	Engine    string `json:"engine,omitempty"` // per-engine token if the flag is engine-scoped
}

// SchemaCommand is a top-level subcommand summary.
type SchemaCommand struct {
	Use   string `json:"use"`
	Short string `json:"short"`
}

// Schema is the machine-readable CLI surface consumed by the config builder
// (tools/configurator.html). It is generated from the live cobra command tree so
// the builder can never drift from the real flags. Emit it with `schema --json`.
type Schema struct {
	Version       string   `json:"version"`
	Command       string   `json:"command"` // the subcommand the flags belong to ("run")
	Engines       []string `json:"engines"`
	Severities    []string `json:"severities"`
	ReportFormats []string `json:"reportFormats"` // union of all engines' unified formats
	// EngineReportFormats lists, per engine token, the report formats THAT engine
	// supports (and its mandatory one) — so the builder shows each scanner only its
	// real options. Source of truth: internal/scanner.
	EngineReportFormats map[string]scanner.EngineFormats `json:"engineReportFormats"`
	Commands            []SchemaCommand                  `json:"commands"`
	Flags               []SchemaFlag                     `json:"flags"`
}

func newSchemaCmd() *cobra.Command {
	var pretty bool
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Print the machine-readable CLI schema (flags/engines) as JSON",
		Long: `schema emits the run command's full flag surface plus the engine, severity, and
report-format enumerations as JSON. It is generated from the live command tree and
is the source of truth for the configuration builder (tools/configurator.html).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s := BuildSchema()
			enc := json.NewEncoder(cmd.OutOrStdout())
			if pretty {
				enc.SetIndent("", "  ")
			}
			enc.SetEscapeHTML(false)
			return enc.Encode(s)
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", true, "pretty-print the JSON (output is always JSON)")
	return cmd
}

// BuildSchema constructs the Schema from the live run command. Exported so the
// generator (hack/gen-configurator) can build the page without shelling out.
func BuildSchema() Schema {
	run := newRunCmd()
	var flags []SchemaFlag
	run.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		group, engine := classifyFlag(f.Name)
		flags = append(flags, SchemaFlag{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Type:      f.Value.Type(),
			Default:   f.DefValue,
			Usage:     f.Usage,
			Group:     group,
			Engine:    engine,
		})
	})

	severities := make([]string, 0, len(model.OrderedSeverities()))
	for _, s := range model.OrderedSeverities() {
		severities = append(severities, string(s))
	}

	// Per-engine supported report formats (keyed by --scanners token).
	tokenEngine := map[string]model.Engine{
		"sast": model.EngineSAST, "sca": model.EngineSCA, "kics": model.EngineIaC,
		"secrets": model.EngineSecrets, "containers": model.EngineContainers,
	}
	engineFormats := map[string]scanner.EngineFormats{}
	for _, tok := range engineTokens {
		engineFormats[tok] = scanner.EngineReportFormats(tokenEngine[tok])
	}

	return Schema{
		Version:             Version,
		Command:             "run",
		Engines:             append([]string(nil), engineTokens...),
		Severities:          severities,
		ReportFormats:       scanner.UnifiedReportFormats(),
		EngineReportFormats: engineFormats,
		Commands: []SchemaCommand{
			{Use: "run", Short: "Run the selected scanners and gate on thresholds"},
			{Use: "validate", Short: "Resolve + validate config and print the native plan without scanning"},
			{Use: "detect", Short: "Print the detected CI system and resolved context"},
			{Use: "schema", Short: "Print the machine-readable CLI schema as JSON"},
			{Use: "version", Short: "Print the version and build info"},
		},
		Flags: flags,
	}
}

// classifyFlag buckets a flag name into a UI group and, when engine-scoped,
// returns its engine token. Keep in sync with registerRunFlags grouping.
func classifyFlag(name string) (group, engine string) {
	// Per-engine flags: <engine>-<suffix> for the known suffixes.
	for _, tok := range engineTokens {
		if name == tok+"-mode" || name == tok+"-path" || name == tok+"-image" {
			return "tool", tok
		}
		if name == tok+"-arg" {
			return "passthrough", tok
		}
		if name == tok+"-report-formats" {
			return "output", tok
		}
		if name == tok+"-queries" {
			return "tool", tok
		}
		if name == tok+"-timeout" {
			return "orchestration", tok
		}
	}
	switch name {
	case "scanners", "source", "project-name", "sast-project-name", "cx-project-name", "branch":
		return "selection", ""
	case "threshold":
		return "threshold", ""
	case "file-filter", "file-include", "use-gitignore",
		"sast-filter", "sca-filter", "iac-security-filter", "kics-filter",
		"containers-file-folder-filter", "containers-package-filter",
		"containers-image-tag-filter", "secrets-filter":
		return "filters", ""
	case "report-formats", "output-path", "output-name", "ignore-on-exit":
		return "output", ""
	case "on-missing", "conflict", "async", "parallel", "fail-fast", "timeout", "dry-run":
		return "orchestration", ""
	case "sca-resolver", "sca-resolver-arg", "container-images":
		if name == "sca-resolver-arg" {
			return "passthrough", "sca"
		}
		return "tool", "sca"
	}
	switch {
	case strings.HasPrefix(name, "cx-"):
		return "auth-cx1", ""
	case strings.HasPrefix(name, "sast-"): // sast-server/user/password/token/sso/java/team
		return "auth-sast", "sast"
	}
	return "other", ""
}
