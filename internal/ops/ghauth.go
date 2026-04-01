package ops

import (
	"os/exec"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// GHUser returns the authenticated GitHub username.
func GHUser() string {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	cmd.Env = append(cmd.Environ(), "GH_HOST="+config.GHHost)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GHIdentity holds the authenticated user's GitHub profile.
type GHIdentity struct {
	Login string
	Name  string
}

// GetGHIdentity returns the authenticated user's login and display name.
func GetGHIdentity() GHIdentity {
	cmd := exec.Command("gh", "api", "user", "--jq", "[.login, .name] | @tsv")
	cmd.Env = append(cmd.Environ(), "GH_HOST="+config.GHHost)
	out, err := cmd.Output()
	if err != nil {
		return GHIdentity{}
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	id := GHIdentity{Login: parts[0]}
	if len(parts) > 1 {
		id.Name = parts[1]
	}
	return id
}

// GHRepoPermission returns the user's permission level on a repo ("admin", "write", "read", "none").
func GHRepoPermission(repo, login string) string {
	if repo == "" || login == "" {
		return "none"
	}
	cmd := exec.Command("gh", "api", "repos/"+repo+"/collaborators/"+login+"/permission", "--jq", ".permission")
	cmd.Env = append(cmd.Environ(), "GH_HOST="+config.GHHost)
	out, err := cmd.Output()
	if err != nil {
		return "none"
	}
	return strings.TrimSpace(string(out))
}

// CanWriteRepo returns true if the user has write or admin access to the repo.
func CanWriteRepo(repo string) bool {
	login := GHUser()
	if login == "" {
		return false
	}
	perm := GHRepoPermission(repo, login)
	return perm == "admin" || perm == "write"
}

// ListForks returns the full_name of all forks of the upstream steer-runtime repo.
func ListForks() ([]string, string) {
	cmd := exec.Command("gh", "api", "repos/"+config.DefaultSteerRepo+"/forks", "--jq", ".[].full_name")
	cmd.Env = append(cmd.Environ(), "GH_HOST="+config.GHHost)
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		return nil, "gh api failed: " + detail
	}
	var forks []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			forks = append(forks, line)
		}
	}
	if len(forks) == 0 {
		return nil, "no forks found for " + config.DefaultSteerRepo
	}
	return forks, ""
}
