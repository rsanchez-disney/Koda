package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const pluginsReleaseURL = "https://github.disney.com/api/v3/repos/SANCR225/steer-plugins/releases/latest"

// IDEInfo represents a detected IDE and its plugin status.
type IDEInfo struct {
	Name      string
	ID        string // vscode, intellij, webstorm
	Installed bool   // IDE is installed on the system
	Plugin    bool   // steer plugin is installed
	Version   string // plugin version if installed
}

// DetectIDEs returns all known IDEs with their install status.
func DetectIDEs() []IDEInfo {
	return []IDEInfo{
		detectVSCode("VS Code", "vscode", "code"),
		detectVSCode("Cursor", "cursor", "cursor"),
		detectIntelliJ("IntelliJ IDEA", "intellij", "IntelliJIdea"),
		detectIntelliJ("WebStorm", "webstorm", "WebStorm"),
		detectIntelliJ("PyCharm", "pycharm", "PyCharm"),
		detectIntelliJ("Rider", "rider", "Rider"),
		detectIntelliJ("GoLand", "goland", "GoLand"),
		detectIntelliJ("Android Studio", "android-studio", "Google/AndroidStudio"),
	}
}

func detectVSCode(name, id, binary string) IDEInfo {
	info := IDEInfo{Name: name, ID: id}
	if _, err := exec.LookPath(binary); err == nil {
		info.Installed = true
		out, err := exec.Command(binary, "--list-extensions").Output()
		if err == nil && strings.Contains(string(out), "steer") {
			info.Plugin = true
			info.Version = "installed"
		}
	}
	return info
}

func detectIntelliJ(name, id, dirPrefix string) IDEInfo {
	info := IDEInfo{Name: name, ID: id}
	pluginsDir := jetbrainsPluginsDir(dirPrefix)
	if pluginsDir != "" {
		info.Installed = true
		steerDir := filepath.Join(pluginsDir, "steer")
		if _, err := os.Stat(steerDir); err == nil {
			info.Plugin = true
			info.Version = "installed"
		}
	}
	return info
}

func jetbrainsPluginsDir(prefix string) string {
	home, _ := os.UserHomeDir()
	var base string
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support", "JetBrains")
	case "windows":
		base = filepath.Join(os.Getenv("APPDATA"), "JetBrains")
	default:
		base = filepath.Join(home, ".local", "share", "JetBrains")
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	// Find latest version dir matching prefix
	var latest string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			latest = filepath.Join(base, e.Name(), "plugins")
		}
	}
	return latest
}

// InstallIDEPlugin downloads and installs the steer plugin for the given IDE.
func InstallIDEPlugin(ide string) error {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".kiro", "tools", "ide-plugins")
	os.MkdirAll(cacheDir, 0755)

	// VS Code family — same .vsix
	switch ide {
	case "vscode":
		return installVSIX(cacheDir, "code")
	case "cursor":
		return installVSIX(cacheDir, "cursor")
	}

	// JetBrains family — same .zip, different plugins dir
	jetbrainsMap := map[string]string{
		"intellij":       "IntelliJIdea",
		"webstorm":       "WebStorm",
		"pycharm":        "PyCharm",
		"rider":          "Rider",
		"goland":         "GoLand",
		"android-studio": "Google/AndroidStudio",
	}
	if prefix, ok := jetbrainsMap[ide]; ok {
		return installJetBrains(cacheDir, prefix, ide)
	}

	return fmt.Errorf("unknown IDE: %s", ide)
}

func installVSIX(cacheDir, binary string) error {
	vsix := filepath.Join(cacheDir, "steer.vsix")
	if err := downloadPluginAsset(vsix, "steer.vsix"); err != nil {
		return err
	}
	fmt.Printf("  Installing via %s...\n", binary)
	return exec.Command(binary, "--install-extension", vsix).Run()
}

func installJetBrains(cacheDir, dirPrefix, ide string) error {
	zip := filepath.Join(cacheDir, "steer.zip")
	if err := downloadPluginAsset(zip, "steer.zip"); err != nil {
		return err
	}
	pluginsDir := jetbrainsPluginsDir(dirPrefix)
	if pluginsDir == "" {
		return fmt.Errorf("%s plugins directory not found", ide)
	}
	fmt.Printf("  Installing to %s...\n", pluginsDir)
	return unzipFile(zip, pluginsDir)
}

func downloadPluginAsset(dest, assetName string) error {
	fmt.Printf("  Downloading %s...\n", assetName)

	// Fetch latest release
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		// Try gh CLI auth
		out, err := exec.Command("gh", "auth", "token", "--hostname", "github.disney.com").Output()
		if err == nil {
			token = strings.TrimSpace(string(out))
		}
	}

	req, _ := http.NewRequest("GET", pluginsReleaseURL, nil)
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	json.NewDecoder(resp.Body).Decode(&rel)

	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			downloadURL = a.URL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("asset %s not found in latest release", assetName)
	}

	// Download
	dlReq, _ := http.NewRequest("GET", downloadURL, nil)
	if token != "" {
		dlReq.Header.Set("Authorization", "token "+token)
	}
	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil {
		return err
	}
	defer dlResp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, dlResp.Body)
	return err
}

func unzipFile(src, dest string) error {
	cmd := exec.Command("unzip", "-o", src, "-d", dest)
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command",
			fmt.Sprintf("Expand-Archive -Force -Path '%s' -DestinationPath '%s'", src, dest))
	}
	return cmd.Run()
}

// IDEPluginCacheDir returns the path where plugin artifacts are cached.
func IDEPluginCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kiro", "tools", "ide-plugins")
}

// CopyPluginFromSteerRuntime copies pre-built plugin from steer-runtime if available.
func CopyPluginFromSteerRuntime(steerRoot string) {
	src := filepath.Join(steerRoot, "shared", "tools", "ide-plugins")
	if _, err := os.Stat(src); err != nil {
		return
	}
	dest := IDEPluginCacheDir()
	os.MkdirAll(dest, 0755)
	for _, name := range []string{"steer.vsix", "steer.zip"} {
		srcFile := filepath.Join(src, name)
		if _, err := os.Stat(srcFile); err == nil {
			copyFile(srcFile, filepath.Join(dest, name))
		}
	}
}
