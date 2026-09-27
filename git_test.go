package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newProjectDir creates an empty project directory for a test.
func newProjectDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// makeRepo initializes a git repository in dir with a deterministic identity, no
// commit signing and one commit on baseBranch. It deliberately does not record
// init.defaultBranch, so tests decide which default branch applies. A local
// commit.gpgsign=true must not be able to make the tests hang on a passphrase
// prompt.
func makeRepo(t *testing.T, dir string, baseBranch string) {
	t.Helper()

	output, err := runGitCommand(dir, "init", "-b", baseBranch)
	if err != nil {
		t.Fatalf("git init failed: %v, output: %s", err, output)
	}

	for key, value := range map[string]string{
		"user.name":      "project-configs test",
		"user.email":     "test@example.com",
		"commit.gpgsign": "false",
		"tag.gpgsign":    "false",
	} {
		if output, err := runGitCommand(dir, "config", key, value); err != nil {
			t.Fatalf("git config %s failed: %v, output: %s", key, err, output)
		}
	}

	writeFile(t, dir, "README.md", "# demo\n")
	gitOutput(t, dir, "add", "-A")
	gitOutput(t, dir, "commit", "-m", "chore: first commit")
}

// recordDefaultBranch records branch as the repository-local init.defaultBranch.
func recordDefaultBranch(t *testing.T, dir string, branch string) {
	t.Helper()
	gitOutput(t, dir, "config", "init.defaultBranch", branch)
}

// configureGitFlowLike records a complete git flow configuration, standing in for
// a repository that was initialized earlier.
func configureGitFlowLike(t *testing.T, dir string, productionBranch string) {
	t.Helper()
	for key, value := range map[string]string{
		"gitflow.branch.master":     productionBranch,
		"gitflow.branch.develop":    gitFlowDevelopBranch,
		"gitflow.prefix.feature":    "feature/",
		"gitflow.prefix.release":    "release/",
		"gitflow.prefix.hotfix":     "hotfix/",
		"gitflow.prefix.support":    "support/",
		"gitflow.prefix.versiontag": "v",
	} {
		gitOutput(t, dir, "config", key, value)
	}
}

// writeFile writes content to a file inside dir, creating parent directories.
func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create directory for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

// writeEmptyGitConfig returns the path of an empty git config file, for use as
// GIT_CONFIG_GLOBAL so the developer's own global config stays untouched.
func writeEmptyGitConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("failed to write the empty global config: %v", err)
	}
	return path
}

// gitOutput runs a git command in dir and returns its trimmed output.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	output, err := runGitCommand(dir, args...)
	if err != nil {
		t.Fatalf("git %s failed: %v, output: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

// commitSubjects lists the commit subjects reachable from ref.
func commitSubjects(t *testing.T, dir string, ref string) []string {
	t.Helper()

	output := gitOutput(t, dir, "log", "--format=%s", ref)
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

// productionBranchOf returns the production release branch git flow settled on.
func productionBranchOf(t *testing.T, dir string) string {
	t.Helper()

	branch := gitOutput(t, dir, "config", "--get", "gitflow.branch.master")
	if branch == "" {
		t.Fatal("gitflow.branch.master is not set")
	}
	return branch
}

// TestGitFlowInitConfiguresGitFlow covers the configuration written instead of
// running `git flow init`: both base branches and the supporting branch prefixes
// must hold exactly the documented values, and the development branch must exist
// so that `git flow` recognizes the repository.
func TestGitFlowInitConfiguresGitFlow(t *testing.T) {
	projectDir := newProjectDir(t)
	writeFile(t, projectDir, "README.md", "# demo\n")

	productionBranch, err := resolveProductionBranch(projectDir)
	if err != nil {
		t.Fatalf("resolveProductionBranch() unexpected error: %v", err)
	}

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	want := map[string]string{
		"gitflow.branch.master":     productionBranch,
		"gitflow.branch.develop":    "develop",
		"gitflow.prefix.feature":    "feature/",
		"gitflow.prefix.release":    "release/",
		"gitflow.prefix.hotfix":     "hotfix/",
		"gitflow.prefix.support":    "support/",
		"gitflow.prefix.versiontag": "v",
	}
	for key, wantValue := range want {
		if got := gitConfigValue(projectDir, key); got != wantValue {
			t.Errorf("%s = %q, want %q", key, got, wantValue)
		}
	}

	// git flow only accepts a repository whose base branches both exist.
	for _, branch := range []string{productionBranch, gitFlowDevelopBranch} {
		exists, err := gitBranchExists(projectDir, branch)
		if err != nil {
			t.Fatalf("gitBranchExists(%q) unexpected error: %v", branch, err)
		}
		if !exists {
			t.Errorf("branch %q does not exist, want it created during git flow configuration", branch)
		}
	}
	if !isGitFlowInitialized(projectDir) {
		t.Error("isGitFlowInitialized() = false, want true after configuration")
	}
	if got := gitCurrentBranch(projectDir); got != gitFlowDevelopBranch {
		t.Errorf("checked out branch = %q, want %q", got, gitFlowDevelopBranch)
	}
}

// TestGitFlowInitLeavesFreshRepositoryOnDevelopBranch covers the handover: a
// repository this tool bootstraps ends up on the development branch, with the
// initial commit reachable from both base branches.
func TestGitFlowInitLeavesFreshRepositoryOnDevelopBranch(t *testing.T) {
	projectDir := newProjectDir(t)
	writeFile(t, projectDir, ".editorconfig", "root = true\n")

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	productionBranch := productionBranchOf(t, projectDir)
	if productionBranch == gitFlowDevelopBranch {
		t.Fatalf("production release branch = %q, want it to differ from the development branch", productionBranch)
	}
	if got := gitCurrentBranch(projectDir); got != gitFlowDevelopBranch {
		t.Errorf("checked out branch = %q, want %q", got, gitFlowDevelopBranch)
	}

	// The initial commit must be present on the branch that is checked out.
	if subjects := commitSubjects(t, projectDir, gitFlowDevelopBranch); len(subjects) != 1 || subjects[0] != defaultGitFlowCommitMessage {
		t.Errorf("commits on %s = %v, want exactly [%q]", gitFlowDevelopBranch, subjects, defaultGitFlowCommitMessage)
	}
	if status := gitOutput(t, projectDir, "status", "--porcelain"); status != "" {
		t.Errorf("git status = %q, want a clean working tree after the handover", status)
	}
}

// TestGitFlowInitKeepsExistingDevelopBranch covers a repository that already has
// a development branch: it must be left alone instead of being recreated.
func TestGitFlowInitKeepsExistingDevelopBranch(t *testing.T) {
	projectDir := newProjectDir(t)
	makeRepo(t, projectDir, "trunk")
	recordDefaultBranch(t, projectDir, "trunk")
	gitOutput(t, projectDir, "branch", gitFlowDevelopBranch)
	before := gitOutput(t, projectDir, "rev-parse", gitFlowDevelopBranch)

	writeFile(t, projectDir, ".editorconfig", "root = true\n")

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	if after := gitOutput(t, projectDir, "rev-parse", gitFlowDevelopBranch); after != before {
		t.Errorf("develop moved from %s to %s, want it untouched", before, after)
	}
	if got := productionBranchOf(t, projectDir); got != "trunk" {
		t.Errorf("gitflow.branch.master = %q, want %q", got, "trunk")
	}
	if got := gitCurrentBranch(projectDir); got != "trunk" {
		t.Errorf("checked out branch = %q, want the existing %q", got, "trunk")
	}
}

// TestGitFlowInitDoesNotCommitExistingRepository covers the rule that a
// repository which already has history is left alone: git flow is configured,
// but the generated files stay uncommitted and no commit is added.
func TestGitFlowInitDoesNotCommitExistingRepository(t *testing.T) {
	projectDir := newProjectDir(t)
	makeRepo(t, projectDir, "trunk")
	recordDefaultBranch(t, projectDir, "trunk")
	before := gitOutput(t, projectDir, "rev-parse", "HEAD")

	writeFile(t, projectDir, ".editorconfig", "root = true\n")

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	// The history is untouched.
	if after := gitOutput(t, projectDir, "rev-parse", "HEAD"); after != before {
		t.Errorf("HEAD moved from %s to %s, want the existing history kept", before, after)
	}
	if subjects := commitSubjects(t, projectDir, "trunk"); len(subjects) != 1 || subjects[0] != "chore: first commit" {
		t.Errorf("commits on trunk = %v, want only the pre-existing commit", subjects)
	}

	// The generated file is left in the working tree for the developer.
	if status := gitOutput(t, projectDir, "status", "--porcelain"); !strings.Contains(status, ".editorconfig") {
		t.Errorf("git status = %q, want the generated file to remain uncommitted", status)
	}

	// The checked-out branch is left alone, so a developer working on a branch
	// is not moved to the development branch underneath them.
	if got := gitCurrentBranch(projectDir); got != "trunk" {
		t.Errorf("checked out branch = %q, want the existing %q", got, "trunk")
	}

	// Git flow is still configured, and releases from the repository's branch.
	if got := productionBranchOf(t, projectDir); got != "trunk" {
		t.Errorf("gitflow.branch.master = %q, want %q", got, "trunk")
	}
	if !isGitFlowInitialized(projectDir) {
		t.Error("isGitFlowInitialized() = false, want the existing repository to be configured")
	}
}

// TestGitFlowInitKeepsConfiguredProductionBranch is the regression test for a
// repository that already releases from a branch of its own: its git flow
// configuration must not be overwritten.
func TestGitFlowInitKeepsConfiguredProductionBranch(t *testing.T) {
	projectDir := newProjectDir(t)
	makeRepo(t, projectDir, "trunk")
	configureGitFlowLike(t, projectDir, "trunk")
	gitOutput(t, projectDir, "branch", gitFlowDevelopBranch)
	// A recorded default branch that disagrees, to prove the configuration wins.
	recordDefaultBranch(t, projectDir, "something-else")

	writeFile(t, projectDir, ".editorconfig", "root = true\n")

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	if got := gitConfigValue(projectDir, "gitflow.branch.master"); got != "trunk" {
		t.Errorf("gitflow.branch.master = %q, want %q", got, "trunk")
	}
}

// TestGitFlowInitCommitsGeneratedFiles covers R3.1/R3.2: the generated files are
// committed to the production release branch with the default message.
func TestGitFlowInitCommitsGeneratedFiles(t *testing.T) {
	projectDir := newProjectDir(t)
	writeFile(t, projectDir, ".editorconfig", "root = true\n")
	writeFile(t, projectDir, ".github/FUNDING.yml", "github: [ansidev]\n")

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	productionBranch := productionBranchOf(t, projectDir)

	if subjects := commitSubjects(t, projectDir, productionBranch); len(subjects) != 1 || subjects[0] != defaultGitFlowCommitMessage {
		t.Errorf("commits on %s = %v, want exactly [%q]", productionBranch, subjects, defaultGitFlowCommitMessage)
	}

	tracked := gitOutput(t, projectDir, "ls-tree", "-r", "--name-only", productionBranch)
	for _, want := range []string{".editorconfig", ".github/FUNDING.yml"} {
		if !strings.Contains(tracked, want) {
			t.Errorf("commit on %s is missing %s, tracked files: %s", productionBranch, want, tracked)
		}
	}
}

// TestGitFlowInitUsesConfiguredCommitMessage covers R3.3: a configured
// commit message replaces the default one.
func TestGitFlowInitUsesConfiguredCommitMessage(t *testing.T) {
	projectDir := newProjectDir(t)
	writeFile(t, projectDir, "README.md", "# demo\n")

	if err := GitFlowInit(projectDir, "chore: bootstrap the project"); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	subjects := commitSubjects(t, projectDir, productionBranchOf(t, projectDir))
	if len(subjects) != 1 || subjects[0] != "chore: bootstrap the project" {
		t.Errorf("commits = %v, want exactly [%q]", subjects, "chore: bootstrap the project")
	}
}

// TestGitFlowInitEmptyRepositoryKeepsConfiguredMessage covers R3.5: even without
// any generated file the initial commit uses the configured message.
func TestGitFlowInitEmptyRepositoryKeepsConfiguredMessage(t *testing.T) {
	projectDir := newProjectDir(t)

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	subjects := commitSubjects(t, projectDir, productionBranchOf(t, projectDir))
	if len(subjects) != 1 || subjects[0] != defaultGitFlowCommitMessage {
		t.Errorf("commits = %v, want exactly [%q]", subjects, defaultGitFlowCommitMessage)
	}
}

// TestGitFlowInitSkipsCommitWhenNothingChanged covers R3.4: a second run on an
// unchanged repository creates no commit and reports the configuration as
// already present.
func TestGitFlowInitSkipsCommitWhenNothingChanged(t *testing.T) {
	projectDir := newProjectDir(t)
	writeFile(t, projectDir, "README.md", "# demo\n")

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("first GitFlowInit() unexpected error: %v", err)
	}
	productionBranch := productionBranchOf(t, projectDir)
	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("second GitFlowInit() unexpected error: %v", err)
	}

	subjects := commitSubjects(t, projectDir, productionBranch)
	if len(subjects) != 1 || subjects[0] != defaultGitFlowCommitMessage {
		t.Errorf("commits = %v, want exactly [%q] after a repeated run", subjects, defaultGitFlowCommitMessage)
	}
}

// TestGitFlowInitSecondRunKeepsFreshRepositoryIdempotent covers a repeated run
// on a repository this tool already bootstrapped: the configuration is reported
// as present and no second commit is created.
func TestGitFlowInitSecondRunKeepsFreshRepositoryIdempotent(t *testing.T) {
	projectDir := newProjectDir(t)
	writeFile(t, projectDir, "README.md", "# demo\n")

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("first GitFlowInit() unexpected error: %v", err)
	}
	productionBranch := productionBranchOf(t, projectDir)
	if !isGitFlowInitialized(projectDir) {
		t.Fatalf("isGitFlowInitialized() = false after the first run in %s", projectDir)
	}
	head := gitOutput(t, projectDir, "rev-parse", "HEAD")

	if err := GitFlowInit(projectDir, ""); err != nil {
		t.Fatalf("second GitFlowInit() unexpected error: %v", err)
	}

	if after := gitOutput(t, projectDir, "rev-parse", "HEAD"); after != head {
		t.Errorf("HEAD moved from %s to %s, want no second commit", head, after)
	}
	if subjects := commitSubjects(t, projectDir, productionBranch); len(subjects) != 1 || subjects[0] != defaultGitFlowCommitMessage {
		t.Errorf("commits = %v, want exactly [%q] after a repeated run", subjects, defaultGitFlowCommitMessage)
	}
}

// TestCommitGeneratedFilesRetargetsUnbornHead covers a repository created outside
// of this tool on a branch other than the recorded default branch: an unborn HEAD
// carries no history, so it can be moved to the production release branch.
func TestCommitGeneratedFilesRetargetsUnbornHead(t *testing.T) {
	projectDir := newProjectDir(t)
	gitOutput(t, projectDir, "init", "-b", "side-quest")
	recordDefaultBranch(t, projectDir, "main")
	writeFile(t, projectDir, ".editorconfig", "root = true\n")

	if err := commitGeneratedFiles(projectDir, "main", defaultGitFlowCommitMessage); err != nil {
		t.Fatalf("commitGeneratedFiles() unexpected error: %v", err)
	}

	if got := gitCurrentBranch(projectDir); got != "main" {
		t.Errorf("HEAD = %q, want %q", got, "main")
	}
	if subjects := commitSubjects(t, projectDir, "main"); len(subjects) != 1 || subjects[0] != defaultGitFlowCommitMessage {
		t.Errorf("commits on main = %v, want exactly [%q]", subjects, defaultGitFlowCommitMessage)
	}
}

func TestResolveProductionBranch(t *testing.T) {
	t.Run("prefers the branch a configured git flow names", func(t *testing.T) {
		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")
		configureGitFlowLike(t, dir, "release")

		got, err := resolveProductionBranch(dir)
		if err != nil {
			t.Fatalf("resolveProductionBranch() unexpected error: %v", err)
		}
		if got != "release" {
			t.Errorf("resolveProductionBranch() = %q, want %q", got, "release")
		}
	})

	t.Run("prefers the repository-local default branch", func(t *testing.T) {
		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")
		recordDefaultBranch(t, dir, "local-choice")

		got, err := resolveProductionBranch(dir)
		if err != nil {
			t.Fatalf("resolveProductionBranch() unexpected error: %v", err)
		}
		if got != "local-choice" {
			t.Errorf("resolveProductionBranch() = %q, want %q", got, "local-choice")
		}
	})

	t.Run("falls back to the global default branch", func(t *testing.T) {
		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")

		global := globalDefaultBranch()
		if global == "" {
			t.Skip("no global init.defaultBranch to fall back to")
		}
		got, err := resolveProductionBranch(dir)
		if err != nil {
			t.Fatalf("resolveProductionBranch() unexpected error: %v", err)
		}
		if got != global {
			t.Errorf("resolveProductionBranch() = %q, want the global default %q", got, global)
		}
	})

	t.Run("records main when neither is configured", func(t *testing.T) {
		// An empty global config stands in for an unconfigured one.
		t.Setenv("GIT_CONFIG_GLOBAL", writeEmptyGitConfig(t))

		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")

		got, err := resolveProductionBranch(dir)
		if err != nil {
			t.Fatalf("resolveProductionBranch() unexpected error: %v", err)
		}
		if got != "main" {
			t.Errorf("resolveProductionBranch() = %q, want %q", got, "main")
		}
		// The decision is recorded so later runs resolve the same branch.
		if recorded := gitConfigValue(dir, "init.defaultBranch"); recorded != "main" {
			t.Errorf("recorded init.defaultBranch = %q, want %q", recorded, "main")
		}
	})
}

func TestIsGitFlowInitialized(t *testing.T) {
	t.Run("repository without git flow config is not initialized", func(t *testing.T) {
		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")

		if isGitFlowInitialized(dir) {
			t.Error("isGitFlowInitialized() = true, want false")
		}
	})

	t.Run("a develop-like branch name is not enough", func(t *testing.T) {
		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")
		gitOutput(t, dir, "branch", "develop-2")

		if isGitFlowInitialized(dir) {
			t.Error("isGitFlowInitialized() = true, want false without gitflow.branch.* config")
		}
	})

	t.Run("matching master and develop are not initialized", func(t *testing.T) {
		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")
		gitOutput(t, dir, "config", "gitflow.branch.master", "trunk")
		gitOutput(t, dir, "config", "gitflow.branch.develop", "trunk")

		if isGitFlowInitialized(dir) {
			t.Error("isGitFlowInitialized() = true, want false when both base branches are equal")
		}
	})

	t.Run("missing prefix is not initialized", func(t *testing.T) {
		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")
		gitOutput(t, dir, "config", "gitflow.branch.master", "trunk")
		gitOutput(t, dir, "config", "gitflow.branch.develop", "develop")

		if isGitFlowInitialized(dir) {
			t.Error("isGitFlowInitialized() = true, want false while a prefix is missing")
		}
	})

	t.Run("distinct base branches and all prefixes are initialized", func(t *testing.T) {
		dir := newProjectDir(t)
		makeRepo(t, dir, "trunk")
		configureGitFlowLike(t, dir, "trunk")

		if !isGitFlowInitialized(dir) {
			t.Error("isGitFlowInitialized() = false, want true")
		}
	})
}

// TestShippedGitFlowResourceConfiguresCommitMessage covers R3.2/R3.3 for the
// config that actually ships with the tool: the gitflow_init resource must stay
// an action resource and must carry the documented commit message.
func TestShippedGitFlowResourceConfiguresCommitMessage(t *testing.T) {
	configs, err := loadConfig("config.yaml")
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}

	group, ok := configs[gitFlowResourceID]
	if !ok {
		t.Fatalf("loadConfig() missing group %q", gitFlowResourceID)
	}
	if len(group.Resources) != 1 {
		t.Fatalf("loadConfig() %q should contain exactly 1 resource, got %d", gitFlowResourceID, len(group.Resources))
	}

	resource := group.Resources[0]
	if resource.ID != gitFlowResourceID {
		t.Errorf("%q resource id = %q, want %q", gitFlowResourceID, resource.ID, gitFlowResourceID)
	}
	// An empty path marks the resource as an action that is not copied.
	if resource.Path != "" {
		t.Errorf("%q path = %q, want an empty path", gitFlowResourceID, resource.Path)
	}
	if resource.CommitMessage != defaultGitFlowCommitMessage {
		t.Errorf("%q commit_message = %q, want %q", gitFlowResourceID, resource.CommitMessage, defaultGitFlowCommitMessage)
	}
}

func TestRunGitCommandRejectsFailure(t *testing.T) {
	dir := newProjectDir(t)

	// A non-zero exit code must be surfaced, not swallowed.
	output, err := runGitCommand(dir, "rev-parse", "--verify", "--quiet", "HEAD")
	if err == nil {
		t.Errorf("runGitCommand() err = nil, want an error outside a repository; output: %s", output)
	}
	if !strings.Contains(fmt.Sprint(err), "exit status") {
		t.Errorf("runGitCommand() err = %v, want the underlying exit status", err)
	}
}
