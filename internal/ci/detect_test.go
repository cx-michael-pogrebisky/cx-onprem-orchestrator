package ci

import "testing"

func envFrom(m map[string]string) EnvFunc {
	return func(k string) string { return m[k] }
}

func TestDetect_Precedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Provider
	}{
		{"github", map[string]string{"GITHUB_ACTIONS": "true", "CI": "true"}, ProviderGitHub},
		{"gitlab", map[string]string{"GITLAB_CI": "true", "CI": "true"}, ProviderGitLab},
		{"azure", map[string]string{"TF_BUILD": "True"}, ProviderAzure},
		{"teamcity", map[string]string{"TEAMCITY_VERSION": "2024.1"}, ProviderTeamCity},
		{"bamboo", map[string]string{"bamboo_buildNumber": "42"}, ProviderBamboo},
		{"bitbucket", map[string]string{"BITBUCKET_BUILD_NUMBER": "7", "CI": "true"}, ProviderBitbucket},
		{"jenkins", map[string]string{"JENKINS_URL": "http://j", "CI": "true"}, ProviderJenkins},
		{"buildkite", map[string]string{"BUILDKITE": "true", "CI": "true"}, ProviderBuildkite},
		{"circleci", map[string]string{"CIRCLECI": "true", "CI": "true"}, ProviderCircleCI},
		{"codebuild", map[string]string{"CODEBUILD_BUILD_ID": "x:1"}, ProviderCodeBuild},
		{"travis", map[string]string{"TRAVIS": "true", "CI": "true"}, ProviderTravis},
		{"drone", map[string]string{"DRONE": "true", "CI": "true"}, ProviderDrone},
		{"semaphore", map[string]string{"SEMAPHORE": "true", "CI": "true"}, ProviderSemaphore},
		{"appveyor", map[string]string{"APPVEYOR": "True", "CI": "True"}, ProviderAppVeyor},
		{"codefresh", map[string]string{"CF_BUILD_ID": "abc"}, ProviderCodefresh},
		{"local", map[string]string{}, ProviderLocal},
		{"ci-true-alone-is-local", map[string]string{"CI": "true"}, ProviderLocal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := Detect(envFrom(tc.env), nil)
			if ctx.Provider != tc.want {
				t.Errorf("Detect provider = %s, want %s", ctx.Provider, tc.want)
			}
		})
	}
}

func TestDetect_GitHubContext(t *testing.T) {
	ctx := Detect(envFrom(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_REF_NAME":   "main",
		"GITHUB_SHA":        "abc123",
		"GITHUB_REPOSITORY": "org/app",
		"GITHUB_SERVER_URL": "https://github.com",
		"GITHUB_WORKSPACE":  "/gh/work",
	}), nil)
	if ctx.Branch != "main" || ctx.Commit != "abc123" || ctx.Workspace != "/gh/work" {
		t.Errorf("unexpected context: %+v", ctx)
	}
	if ctx.Repo != "https://github.com/org/app" {
		t.Errorf("repo = %q", ctx.Repo)
	}
}

func TestDetect_AzureStripsRef(t *testing.T) {
	ctx := Detect(envFrom(map[string]string{
		"TF_BUILD":           "True",
		"BUILD_SOURCEBRANCH": "refs/heads/feature/tools",
	}), nil)
	if ctx.Branch != "feature/tools" {
		t.Errorf("want stripped branch feature/tools, got %q", ctx.Branch)
	}
}

func TestDetect_TeamCityGitFallback(t *testing.T) {
	gitStub := func(workdir string) (string, string, string) {
		return "develop", "deadbeef", "git@host:org/app.git"
	}
	ctx := Detect(envFrom(map[string]string{
		"TEAMCITY_VERSION": "2024.1",
		"BUILD_VCS_NUMBER": "deadbeef",
	}), gitStub)
	if ctx.Branch != "develop" || ctx.Repo != "git@host:org/app.git" {
		t.Errorf("teamcity git fallback failed: %+v", ctx)
	}
	if len(ctx.Notes) == 0 {
		t.Errorf("expected fallback notes")
	}
}
