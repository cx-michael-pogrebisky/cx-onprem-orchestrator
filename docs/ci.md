# CI integration

`cx-onprem-orchestrator` auto-detects the CI system and extracts branch / commit /
repo / workspace, so you usually only pass `--scanners` and `--threshold`. The
single static binary (or the fat image) runs on every system below.

Detection precedence (avoids the shared `CI=true` collision):
`GITHUB_ACTIONS → GITLAB_CI → TF_BUILD (Azure) → TEAMCITY_VERSION → bamboo_* →
BITBUCKET_BUILD_NUMBER → JENKINS_URL → local`. Run `cx-onprem-orchestrator detect`
to see what it resolved.

Provide secrets as environment variables (never on the command line):
`CX1_APIKEY` (SCA/containers), `CXSAST_URL` / `CXSAST_USERNAME` / `CXSAST_PASSWORD`
(CxSAST). Reports survive a threshold breach (the report-collection barrier), so
upload them with an always-run step.

## Ready-to-paste pipeline snippets (one per system)

| CI system | Snippet |
|---|---|
| GitHub Actions | [ci/github.md](ci/github.md) |
| GitLab CI | [ci/gitlab.md](ci/gitlab.md) |
| Azure DevOps Pipelines | [ci/azure.md](ci/azure.md) |
| Jenkins (declarative) | [ci/jenkins.md](ci/jenkins.md) |
| Bamboo | [ci/bamboo.md](ci/bamboo.md) |
| Bitbucket Pipelines | [ci/bitbucket.md](ci/bitbucket.md) |
| TeamCity | [ci/teamcity.md](ci/teamcity.md) |

All seven are covered. Each file contains the auto-detection notes, the secret
environment variables, and a copy-paste pipeline definition for that system.
