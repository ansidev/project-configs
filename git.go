package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
)

const (
	// gitFlowResourceID marks the config resource that triggers the GitFlow
	// initialization instead of a file copy. The GitFlow step is always
	// executed after every other selected resource has been copied.
	gitFlowResourceID = "gitflow_init"

	// defaultGitFlowCommitMessage is the commit message used when the
	// gitflow_init resource does not configure one.
	defaultGitFlowCommitMessage = "chore: initial commit"

	// fallbackGitDefaultBranch is recorded in the repository when neither a
	// local nor a global init.defaultBranch is configured.
	fallbackGitDefaultBranch = "main"

	// gitFlowDevelopBranch is the branch git flow integrates the next release
	// on.
	gitFlowDevelopBranch = "develop"

	// gitFlowVersionTagPrefix is put in front of the version in release tags.
	gitFlowVersionTagPrefix = "v"
)

// gitFlowBranchPrefixes are the supporting branch prefixes git flow uses. They
// are written in one order so the configuration is stable and reviewable.
var gitFlowBranchPrefixes = []struct {
	key   string
	value string
}{
	{key: "gitflow.prefix.feature", value: "feature/"},
	{key: "gitflow.prefix.release", value: "release/"},
	{key: "gitflow.prefix.hotfix", value: "hotfix/"},
	{key: "gitflow.prefix.support", value: "support/"},
	{key: "gitflow.prefix.versiontag", value: gitFlowVersionTagPrefix},
}

// GitFlowInit initializes git and configures git flow for a project and commits
// the generated files to the production release branch.
//
// git flow is configured directly instead of through `git flow init`, which is
// interactive and, on git-flow 0.4.1, cannot adopt a repository that already
// holds several local branches.
//
// A repository without commits gets them as its root commit, written before the
// configuration so that the development branch has a commit to fork from, and is
// handed over on the development branch. A repository that already has history
// is left alone: neither the history nor the checked-out branch changes and the
// generated files stay uncommitted.
//
// commitMessage is the message of that commit; pass an empty string to use
// defaultGitFlowCommitMessage.
func GitFlowInit(projectPath string, commitMessage string) error {
	var results []string

	if strings.TrimSpace(commitMessage) == "" {
		commitMessage = defaultGitFlowCommitMessage
	}

	// Check if git repository already exists
	gitInitDone, err := isGitRepoInitialized(projectPath)
	if err != nil {
		return fmt.Errorf("failed to check git repository status: %v", err)
	}

	if !gitInitDone {
		// Initialize the repository with git's own default branch, so the
		// initial branch follows the developer's configuration untouched.
		err = runGitInit(projectPath)
		if err != nil {
			return fmt.Errorf("failed to initialize git repository: %v", err)
		}
		results = append(results, "✓ Git repository initialized")
	} else {
		results = append(results, "○ Git repository already initialized (skipped)")
	}

	productionBranch, err := resolveProductionBranch(projectPath)
	if err != nil {
		return err
	}

	// Only a fresh repository is committed and left on the development branch. An
	// existing repository keeps its history and its checked-out branch untouched,
	// and the generated files stay uncommitted for the developer to handle.
	isFreshRepository := !gitHasCommits(projectPath)

	if isFreshRepository {
		if err := commitGeneratedFiles(projectPath, productionBranch, commitMessage); err != nil {
			return err
		}
		results = append(results, fmt.Sprintf("✓ Generated files committed to %s", pterm.Green(productionBranch)))
	} else {
		results = append(results, "○ Existing history kept, generated files not committed")
	}

	// Configure git flow
	if isGitFlowInitialized(projectPath) {
		results = append(results, "○ Git Flow already initialized (skipped)")
	} else {
		err = configureGitFlow(projectPath, productionBranch)
		if err != nil {
			return fmt.Errorf("failed to configure git flow: %v", err)
		}
		results = append(results, fmt.Sprintf("✓ Git Flow initialized (%s → %s)", pterm.Green(productionBranch), pterm.Green(gitFlowDevelopBranch)))
	}

	// Work happens on the development branch, so a fresh repository is handed
	// over there instead of on the branch its initial commit landed on. The
	// branch is ensured first, because a repository that was already configured
	// never went through configureGitFlow.
	if isFreshRepository {
		if err := ensureDevelopBranch(projectPath, productionBranch); err != nil {
			return err
		}
		if err := checkoutBranch(projectPath, gitFlowDevelopBranch); err != nil {
			return err
		}
		results = append(results, fmt.Sprintf("✓ Switched to %s", pterm.Green(gitFlowDevelopBranch)))
	}

	// Print summary
	pterm.Println()
	pterm.Info.Println("GitFlow Initialization Summary:")
	for _, result := range results {
		pterm.Printfln("  %s", result)
	}

	return nil
}

// resolveProductionBranch returns the branch that carries production releases.
//
// A repository that already configures git flow keeps its own production branch.
// Otherwise the default branch of the repository decides: the repository-local
// `init.defaultBranch` first, then the developer's global one, and finally
// `main`, which is recorded in the repository so later runs are stable.
func resolveProductionBranch(projectPath string) (string, error) {
	if branch := gitConfigValue(projectPath, "gitflow.branch.master"); branch != "" {
		return branch, nil
	}

	if branch := gitConfigValue(projectPath, "init.defaultBranch"); branch != "" {
		return branch, nil
	}

	if branch := globalDefaultBranch(); branch != "" {
		return branch, nil
	}

	output, err := runGitCommand(projectPath, "config", "init.defaultBranch", fallbackGitDefaultBranch)
	if err != nil {
		return "", fmt.Errorf("failed to record init.defaultBranch=%s: %v, output: %s", fallbackGitDefaultBranch, err, output)
	}

	return fallbackGitDefaultBranch, nil
}

// configureGitFlow writes the git flow configuration for the project. The
// production release branch becomes productionBranch and the supporting branch
// prefixes get git flow's usual names.
func configureGitFlow(projectPath string, productionBranch string) error {
	pterm.Printfln("Configuring git flow in %s", pterm.Green(projectPath))

	settings := [][2]string{
		{"gitflow.branch.master", productionBranch},
		{"gitflow.branch.develop", gitFlowDevelopBranch},
	}
	for _, prefix := range gitFlowBranchPrefixes {
		settings = append(settings, [2]string{prefix.key, prefix.value})
	}

	for _, setting := range settings {
		output, err := runGitCommand(projectPath, "config", setting[0], setting[1])
		if err != nil {
			return fmt.Errorf("failed to set %s=%s: %v, output: %s", setting[0], setting[1], err, output)
		}
	}

	return ensureDevelopBranch(projectPath, productionBranch)
}

// ensureDevelopBranch creates the git flow development branch from the
// production release branch when it is missing.
//
// git flow only treats a repository as initialized when both base branches
// exist, and it verifies them with the same check the configuration is read
// with. Without the branch every `git flow` command fails with "Not a
// gitflow-enabled repo yet", which is exactly what `git flow init` would have
// created.
func ensureDevelopBranch(projectPath string, productionBranch string) error {
	exists, err := gitBranchExists(projectPath, gitFlowDevelopBranch)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	output, err := runGitCommand(projectPath, "branch", gitFlowDevelopBranch, productionBranch)
	if err != nil {
		return fmt.Errorf("failed to create branch %q from %q: %v, output: %s", gitFlowDevelopBranch, productionBranch, err, output)
	}

	return nil
}

// isGitRepoInitialized checks if a git repository is already initialized
func isGitRepoInitialized(projectPath string) (bool, error) {
	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// runGitInit executes git init in the project directory. The initial branch is
// whatever git itself picks, so the developer's own configuration decides it.
func runGitInit(projectPath string) error {
	pterm.Printfln("Running: %s in %s", pterm.Green("git init"), pterm.Green(projectPath))

	output, err := runGitCommand(projectPath, "init")
	if err != nil {
		return fmt.Errorf("git init failed: %v, output: %s", err, output)
	}

	return nil
}

// isGitFlowInitialized reports whether git flow is already configured, using the
// same condition as git flow itself: both base branches are configured and
// differ, and every supporting branch prefix is set.
func isGitFlowInitialized(projectPath string) bool {
	masterBranch := gitConfigValue(projectPath, "gitflow.branch.master")
	if masterBranch == "" {
		return false
	}

	developBranch := gitConfigValue(projectPath, "gitflow.branch.develop")
	if developBranch == "" || developBranch == masterBranch {
		return false
	}

	for _, prefix := range gitFlowBranchPrefixes {
		if gitConfigValue(projectPath, prefix.key) == "" {
			return false
		}
	}

	return true
}

// checkoutBranch switches the repository to the given branch.
func checkoutBranch(projectPath string, branch string) error {
	pterm.Printfln("Running: %s in %s", pterm.Green("git checkout "+branch), pterm.Green(projectPath))

	output, err := runGitCommand(projectPath, "checkout", branch)
	if err != nil {
		return fmt.Errorf("failed to switch to branch %q: %v, output: %s", branch, err, output)
	}

	return nil
}

// commitGeneratedFiles stages every generated file and commits it to the
// production release branch. It is only called for a repository without commits,
// so the result is the root commit of the repository.
//
// The commit is allowed to be empty because git flow needs the production
// release branch to carry one before the development branch can fork off it.
func commitGeneratedFiles(projectPath string, branch string, message string) error {
	currentBranch := gitCurrentBranch(projectPath)

	// An unborn HEAD carries no history, so it can be pointed at the production
	// release branch. This keeps the recorded default branch authoritative even
	// for a repository created outside of this tool.
	if currentBranch != "" && currentBranch != branch {
		output, err := runGitCommand(projectPath, "symbolic-ref", "HEAD", "refs/heads/"+branch)
		if err != nil {
			return fmt.Errorf("failed to point HEAD at branch %q: %v, output: %s", branch, err, output)
		}
	}

	if output, err := runGitCommand(projectPath, "add", "-A"); err != nil {
		return fmt.Errorf("failed to stage the generated files: %v, output: %s", err, output)
	}

	output, err := runGitCommand(projectPath, "commit", "-m", message, "--allow-empty")
	if err != nil {
		return fmt.Errorf("failed to commit the generated files: %v, output: %s", err, output)
	}

	return nil
}

// globalDefaultBranch returns the developer's global init.defaultBranch, or an
// empty string when it is not configured.
func globalDefaultBranch() string {
	output, err := runGitCommand("", "config", "--global", "--get", "init.defaultBranch")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// gitHasCommits reports whether the repository has at least one commit. An
// unborn HEAD reports false, which is the expected result for a repository this
// tool is about to populate.
func gitHasCommits(projectPath string) bool {
	output, err := runGitCommand(projectPath, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) != ""
}

// gitBranchExists reports whether the given local branch exists.
func gitBranchExists(projectPath string, branch string) (bool, error) {
	output, err := runGitCommand(projectPath, "branch", "--list", branch)
	if err != nil {
		return false, fmt.Errorf("failed to list branches: %v, output: %s", err, output)
	}

	return strings.TrimSpace(string(output)) != "", nil
}

// gitCurrentBranch returns the branch HEAD points at, or an empty string when
// HEAD is detached. Unlike `git rev-parse --abbrev-ref HEAD` it also works on an
// unborn branch, which is what a freshly initialized repository has.
func gitCurrentBranch(projectPath string) string {
	output, err := runGitCommand(projectPath, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// gitConfigValue returns a repository-local git config value, or an empty
// string when the key is not set, which git reports with a non-zero exit code.
func gitConfigValue(projectPath string, key string) string {
	output, err := runGitCommand(projectPath, "config", "--get", key)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// runGitCommand runs a git command in the given directory and returns its
// combined output. An empty directory runs the command in the working
// directory of the process.
func runGitCommand(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	return output, err
}
