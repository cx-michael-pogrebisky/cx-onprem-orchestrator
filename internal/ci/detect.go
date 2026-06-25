// Package ci auto-detects the CI system and extracts repo/branch/commit/
// workspace context. Detection uses a precedence chain that avoids the shared
// CI=true collision (Jenkins/GitLab/Bitbucket all set it). Detected values feed a
// viper DEFAULT layer, so explicit flags/env/config always override them.
package ci

import "strings"

// Provider identifies a CI system (or "local" when none is detected).
type Provider string

const (
	ProviderGitHub    Provider = "github"
	ProviderGitLab    Provider = "gitlab"
	ProviderAzure     Provider = "azure"
	ProviderTeamCity  Provider = "teamcity"
	ProviderBamboo    Provider = "bamboo"
	ProviderBitbucket Provider = "bitbucket"
	ProviderJenkins   Provider = "jenkins"
	ProviderLocal     Provider = "local"
)

// Context is the resolved CI context.
type Context struct {
	Provider  Provider
	Branch    string
	Commit    string
	Repo      string
	Workspace string
	// Notes records provenance / fallbacks for `cx-onprem-orchestrator detect`.
	Notes []string
}

// EnvFunc looks up an environment variable (os.Getenv in production; a stub in tests).
type EnvFunc func(string) string

// GitFunc resolves branch/commit/repo via git when CI env vars are absent
// (used for the TeamCity fallback). Returns empty strings if git is unavailable.
type GitFunc func(workdir string) (branch, commit, repo string)

// Detect runs the precedence chain and extracts context for the detected provider.
func Detect(env EnvFunc, git GitFunc) Context {
	set := func(k string) bool { return strings.TrimSpace(env(k)) != "" }

	switch {
	case strings.EqualFold(env("GITHUB_ACTIONS"), "true"):
		return Context{
			Provider:  ProviderGitHub,
			Branch:    firstNonEmpty(env("GITHUB_HEAD_REF"), env("GITHUB_REF_NAME")),
			Commit:    env("GITHUB_SHA"),
			Repo:      joinGitHubRepo(env("GITHUB_SERVER_URL"), env("GITHUB_REPOSITORY")),
			Workspace: env("GITHUB_WORKSPACE"),
		}
	case strings.EqualFold(env("GITLAB_CI"), "true"):
		return Context{
			Provider:  ProviderGitLab,
			Branch:    firstNonEmpty(env("CI_COMMIT_BRANCH"), env("CI_COMMIT_REF_NAME")),
			Commit:    env("CI_COMMIT_SHA"),
			Repo:      firstNonEmpty(env("CI_PROJECT_URL"), env("CI_REPOSITORY_URL")),
			Workspace: env("CI_PROJECT_DIR"),
		}
	case strings.EqualFold(env("TF_BUILD"), "true"):
		return Context{
			Provider:  ProviderAzure,
			Branch:    refName(firstNonEmpty(env("BUILD_SOURCEBRANCH"), env("BUILD_SOURCEBRANCHNAME"))),
			Commit:    env("BUILD_SOURCEVERSION"),
			Repo:      env("BUILD_REPOSITORY_URI"),
			Workspace: firstNonEmpty(env("BUILD_SOURCESDIRECTORY"), env("SYSTEM_DEFAULTWORKINGDIRECTORY")),
		}
	case set("TEAMCITY_VERSION"):
		return detectTeamCity(env, git)
	case hasBambooVars(env):
		return Context{
			Provider:  ProviderBamboo,
			Branch:    env("bamboo_planRepository_branchName"),
			Commit:    env("bamboo_planRepository_revision"),
			Repo:      env("bamboo_planRepository_repositoryUrl"),
			Workspace: env("bamboo_build_working_directory"),
		}
	case set("BITBUCKET_BUILD_NUMBER"):
		return Context{
			Provider:  ProviderBitbucket,
			Branch:    firstNonEmpty(env("BITBUCKET_BRANCH"), env("BITBUCKET_TAG")),
			Commit:    env("BITBUCKET_COMMIT"),
			Repo:      firstNonEmpty(env("BITBUCKET_GIT_HTTP_ORIGIN"), env("BITBUCKET_REPO_FULL_NAME")),
			Workspace: env("BITBUCKET_CLONE_DIR"),
		}
	case set("JENKINS_URL") || set("JENKINS_HOME"):
		return Context{
			Provider:  ProviderJenkins,
			Branch:    refName(firstNonEmpty(env("GIT_LOCAL_BRANCH"), env("GIT_BRANCH"))),
			Commit:    env("GIT_COMMIT"),
			Repo:      env("GIT_URL"),
			Workspace: env("WORKSPACE"),
		}
	default:
		return Context{Provider: ProviderLocal}
	}
}

// detectTeamCity handles the TeamCity gotcha: only BUILD_VCS_NUMBER is reliably
// exported as an env var; branch/repo/checkout are configuration parameters that
// are NOT auto-exported. We read the CXSCAN_* fallbacks users may wire, else fall
// back to git introspection, recording a note.
func detectTeamCity(env EnvFunc, git GitFunc) Context {
	ctx := Context{
		Provider:  ProviderTeamCity,
		Commit:    env("BUILD_VCS_NUMBER"),
		Workspace: env("CXSCAN_WORKSPACE"),
		Branch:    env("CXSCAN_BRANCH"),
		Repo:      env("CXSCAN_REPO"),
	}
	if (ctx.Branch == "" || ctx.Repo == "" || ctx.Commit == "") && git != nil {
		b, c, r := git(ctx.Workspace)
		if ctx.Branch == "" && b != "" {
			ctx.Branch = b
			ctx.Notes = append(ctx.Notes, "TeamCity: branch not in env; fell back to `git rev-parse --abbrev-ref HEAD`")
		}
		if ctx.Commit == "" && c != "" {
			ctx.Commit = c
		}
		if ctx.Repo == "" && r != "" {
			ctx.Repo = r
			ctx.Notes = append(ctx.Notes, "TeamCity: repo not in env; fell back to `git config --get remote.origin.url`")
		}
		ctx.Notes = append(ctx.Notes,
			"TeamCity: to set these explicitly, add env.CXSCAN_BRANCH=%teamcity.build.branch% and env.CXSCAN_REPO=%vcsroot.url% to your build config")
	}
	return ctx
}

func hasBambooVars(env EnvFunc) bool {
	for _, k := range []string{"bamboo_buildNumber", "bamboo_planKey", "bamboo_buildKey", "bamboo_build_working_directory"} {
		if strings.TrimSpace(env(k)) != "" {
			return true
		}
	}
	return false
}

// refName strips a leading refs/heads/ or refs/tags/ or origin/ prefix to yield a
// short branch name.
func refName(ref string) string {
	r := strings.TrimSpace(ref)
	for _, p := range []string{"refs/heads/", "refs/tags/", "refs/remotes/"} {
		r = strings.TrimPrefix(r, p)
	}
	r = strings.TrimPrefix(r, "origin/")
	return r
}

func joinGitHubRepo(server, repo string) string {
	if repo == "" {
		return ""
	}
	if server == "" {
		server = "https://github.com"
	}
	return strings.TrimRight(server, "/") + "/" + repo
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
