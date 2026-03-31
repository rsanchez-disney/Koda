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
