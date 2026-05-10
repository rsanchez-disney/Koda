package ops

import (
	"archive/tar"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
	"golang.org/x/crypto/pbkdf2"
)

const steerReleaseAPI = "https://api.github.com/repos/rsanchez-disney/steer-runtime/releases/latest"

// releaseKey is set at build time via -ldflags
var releaseKey string

// GetReleaseKey returns the build-time embedded release key.
func GetReleaseKey() string { return releaseKey }

type releaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// SyncSteerRuntime fetches latest changes based on source type, then re-installs profiles.
func SyncSteerRuntime(steerRoot, targetDir string) error {
	settings := config.ReadSteerSettings()

	// Safety: if steerRoot has .git, always use git sync regardless of settings
	hasGit := false
	if _, err := os.Stat(filepath.Join(steerRoot, ".git")); err == nil {
		hasGit = true
	}

	if settings.Source == "git" || hasGit {
		if err := syncGit(steerRoot); err != nil {
			return err
		}
	} else {
		if err := DownloadFromRelease(steerRoot); err != nil {
			return err
		}
	}

	config.MarkSynced()

	// Re-install profiles, respecting active workspace overrides
	installed := DetectInstalled(steerRoot, targetDir)
	InstallShared(steerRoot, targetDir)
	CleanStaleKiroConfig()
	InstallProfile(steerRoot, "core", targetDir)
	for _, p := range installed {
		// Install global profile first, then overlay workspace agents (if any).
		// This matches the workspace apply flow in workspaces.go.
		InstallProfile(steerRoot, p, targetDir)
		if settings.ActiveWorkspace != "" {
			wsDir := filepath.Join(findWorkspaceDir(steerRoot, settings.ActiveWorkspace), "profiles", p)
			if _, err := os.Stat(wsDir); err == nil {
				InstallProfileFrom(wsDir, targetDir)
			}
		}
	}
	InjectAgentTokens(targetDir)

	// Re-apply active workspace files (context, rules, snapshot) so they survive the refresh
	if settings.ActiveWorkspace != "" {
		if ws, err := GetWorkspace(steerRoot, settings.ActiveWorkspace); err == nil {
			resolved, wsNames := ResolveWorkspace(steerRoot, ws)
			RefreshWorkspaceFiles(steerRoot, targetDir, resolved, wsNames)
		}
	}

	// Build context index for RAG-based retrieval
	contextDir := filepath.Join(targetDir, config.ContextDir)
	BuildContextIndex(contextDir) // best-effort, errors ignored

	return nil
}

func syncGit(steerRoot string) error {
	// Check for local uncommitted changes before pulling
	status := exec.Command("git", "-C", steerRoot, "status", "--porcelain")
	out, _ := status.Output()
	dirty := strings.TrimSpace(string(out))

	if dirty != "" {
		lines := strings.Split(dirty, "\n")
		action := handleDirtySync(steerRoot, lines)
		switch action {
		case syncActionStash:
			exec.Command("git", "-C", steerRoot, "stash", "push", "-m", "koda-sync-auto-stash").Run()
			defer func() {
				exec.Command("git", "-C", steerRoot, "stash", "pop").Run()
				logln("  ✓ Local changes restored from stash")
			}()
		case syncActionCommit:
			if err := commitLocalChanges(steerRoot); err != nil {
				return fmt.Errorf("commit failed: %w", err)
			}
			return nil // commitLocalChanges already pulls
		case syncActionDiscard:
			exec.Command("git", "-C", steerRoot, "checkout", "--", ".").Run()
			exec.Command("git", "-C", steerRoot, "clean", "-fd").Run()
			logln("  ✓ Local changes discarded")
		case syncActionAbort:
			return fmt.Errorf("sync aborted by user")
		}
	}

	cmd := exec.Command("git", "-C", steerRoot, "pull", "--ff-only")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		// ff-only failed — try rebase
		cmd2 := exec.Command("git", "-C", steerRoot, "pull", "--rebase")
		cmd2.Stdout = nil
		cmd2.Stderr = nil
		return cmd2.Run()
	}
	return nil
}

// syncAction represents the user's choice when local changes are detected.
type syncAction int

const (
	syncActionStash   syncAction = iota // stash, pull, pop (default)
	syncActionCommit                     // commit to branch, then pull
	syncActionDiscard                    // discard all local changes
	syncActionAbort                      // cancel sync
)

// handleDirtySync shows local changes and prompts the user for action.
// In non-interactive mode (no TTY), defaults to stash.
func handleDirtySync(steerRoot string, changes []string) syncAction {
	logf("  ⚠️  %d local changes detected:\n\n", len(changes))
	for _, line := range changes {
		logf("    %s\n", line)
	}
	logln("")

	if !isInteractive() {
		logln("  Non-interactive — auto-stashing")
		return syncActionStash
	}

	logln("  [s] Stash & sync (restore after)  — default")
	logln("  [d] Show diff")
	logln("  [c] Commit to branch, then sync")
	logln("  [r] Discard all changes & sync")
	logln("  [q] Abort sync")
	logln("")

	for {
		fmt.Print("  Choose [s/d/c/r/q] (default: s): ")
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "", "s":
			return syncActionStash
		case "d":
			showDiff(steerRoot)
			continue // re-prompt after showing diff
		case "c":
			return syncActionCommit
		case "r":
			return syncActionDiscard
		case "q":
			return syncActionAbort
		default:
			logln("  Invalid choice. Use s, d, c, r, or q.")
		}
	}
}

// showDiff runs git diff and prints it.
func showDiff(steerRoot string) {
	cmd := exec.Command("git", "-C", steerRoot, "diff")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	// Also show untracked files
	cmd2 := exec.Command("git", "-C", steerRoot, "diff", "--cached")
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	cmd2.Run()
	logln("")
}

// isInteractive returns true if stdin is a terminal.
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// commitLocalChanges creates a branch, commits local changes, and optionally creates a PR.
func commitLocalChanges(steerRoot string) error {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	branch := "feat/local-changes-" + ts
	
	// Create branch
	exec.Command("git", "-C", steerRoot, "checkout", "-b", branch).Run()
	
	// Stage and commit
	exec.Command("git", "-C", steerRoot, "add", "-A").Run()
	msg := "feat: local workspace/agent changes"
	exec.Command("git", "-C", steerRoot, "commit", "-m", msg).Run()
	
	// Push
	if err := exec.Command("git", "-C", steerRoot, "push", "-u", "origin", branch).Run(); err != nil {
		logf("  ⚠ Push failed: %v\n", err)
		logf("  Changes committed to local branch: %s\n", branch)
		return nil
	}
	
	logf("  ✓ Changes committed and pushed to branch: %s\n", branch)
	logln("  💡 Create a PR with: gh pr create --base main")
	
	// Return to main for sync
	exec.Command("git", "-C", steerRoot, "checkout", "main").Run()
	
	// Now pull
	cmd := exec.Command("git", "-C", steerRoot, "pull", "--ff-only")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// ForkSteerRuntime replaces the tarball install with a git clone of the given fork.
func ForkSteerRuntime(steerRoot, repo, branch string) error {
	url := GitCloneURL(repo)
	os.RemoveAll(steerRoot)
	cmd := exec.Command("git", "clone", "--depth", "1", "-b", branch, url, steerRoot)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(out)))
	}

	s := config.ReadSteerSettings()
	s.Source = "git"
	s.Repo = repo
	s.Branch = branch
	config.SaveSteerSettings(s)
	config.MarkSynced()
	return nil
}

// UnforkSteerRuntime switches back to the canonical tarball source.
func UnforkSteerRuntime(steerRoot string) error {
	os.RemoveAll(steerRoot)
	if err := DownloadFromRelease(steerRoot); err != nil {
		return fmt.Errorf("tarball download failed: %w", err)
	}

	s := config.ReadSteerSettings()
	s.Source = "tarball"
	s.Repo = config.DefaultSteerRepo
	s.Branch = config.DefaultSteerBranch
	config.SaveSteerSettings(s)
	config.MarkSynced()
	return nil
}

// --- Tarball download (moved from cli/bootstrap.go) ---

func DownloadFromRelease(dir string) error {
	rel, err := fetchLatestRelease()
	if err != nil {
		return err
	}

	url, encrypted := findTarball(rel)
	if url == "" {
		return fmt.Errorf("no .tar.gz asset found in release %s", rel.TagName)
	}

	if encrypted && releaseKey == "" {
		return fmt.Errorf("encrypted release but no STEER_RELEASE_KEY compiled into this build")
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tarData []byte
	if encrypted && len(data) > 8 && string(data[:8]) == "Salted__" {
		tarData, err = DecryptOpenSSL(data, releaseKey)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
	} else {
		tarData = data
	}

	// Refuse to nuke a directory that contains .git (or is a symlink to one)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return fmt.Errorf("refusing to overwrite git repo at %s", dir)
	}
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	if err := ExtractTarGz(tarData, dir); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	os.WriteFile(filepath.Join(dir, "VERSION"), []byte(rel.TagName), 0644)
	return nil
}

func fetchLatestRelease() (*releaseInfo, error) {
	resp, err := http.Get(steerReleaseAPI)
	if err != nil {
		return nil, fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func findTarball(rel *releaseInfo) (url string, encrypted bool) {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz") && !strings.HasSuffix(a.Name, ".tar.gz.enc") {
			return a.URL, false
		}
	}
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz.enc") {
			return a.URL, true
		}
	}
	return "", false
}

func DecryptOpenSSL(data []byte, passphrase string) ([]byte, error) {
	if len(data) < 16 || string(data[:8]) != "Salted__" {
		return nil, fmt.Errorf("not an OpenSSL encrypted file")
	}
	salt := data[8:16]
	ciphertext := data[16:]

	keyIV := pbkdf2.Key([]byte(passphrase), salt, 10000, 48, sha256.New)
	key := keyIV[:32]
	iv := keyIV[32:48]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	if len(plaintext) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	padLen := int(plaintext[len(plaintext)-1])
	if padLen > aes.BlockSize || padLen == 0 {
		return nil, fmt.Errorf("invalid padding — wrong STEER_RELEASE_KEY?")
	}
	for i := 0; i < padLen; i++ {
		if plaintext[len(plaintext)-1-i] != byte(padLen) {
			return nil, fmt.Errorf("corrupt padding — wrong STEER_RELEASE_KEY?")
		}
	}
	return plaintext[:len(plaintext)-padLen], nil
}

func ExtractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			io.Copy(f, tr)
			f.Close()
			os.Chmod(target, os.FileMode(hdr.Mode))
		}
	}
	return nil
}

// DisplaySteerReleaseNotes reads RELEASE_NOTES.md from steer-runtime and prints
// the latest version block (between <!-- LATEST --> and <!-- END LATEST --> markers).
func DisplaySteerReleaseNotes(steerRoot string) {
	data, err := os.ReadFile(filepath.Join(steerRoot, "RELEASE_NOTES.md"))
	if err != nil {
		return
	}
	content := string(data)
	start := strings.Index(content, "<!-- LATEST -->")
	end := strings.Index(content, "<!-- END LATEST -->")
	if start < 0 || end < 0 || end <= start {
		return
	}
	block := strings.TrimSpace(content[start+len("<!-- LATEST -->") : end])
	if block == "" {
		return
	}
	fmt.Println("\n📋 What's new in steer-runtime:")
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			fmt.Println("  " + line)
		} else if strings.HasPrefix(line, "## ") {
			fmt.Println("  " + line[3:])
		}
	}
	fmt.Println()
}
