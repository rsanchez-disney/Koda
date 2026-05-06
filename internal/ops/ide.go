package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const pluginsReleaseURL = "https://api.github.com/repos/rsanchez-disney/Koda/releases/latest"

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
	} else if resolveAppBinary(binary) != "" {
		info.Installed = true
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
	if _, err := exec.LookPath(binary); err != nil {
		if resolved := resolveAppBinary(binary); resolved != "" {
			fmt.Printf("  '%s' not in PATH, creating symlink...\n", binary)
			dst := "/usr/local/bin/" + binary
			if err := os.Symlink(resolved, dst); err != nil {
				// Try with sudo
				if err := exec.Command("sudo", "ln", "-s", resolved, dst).Run(); err != nil {
					return fmt.Errorf("'%s' not in PATH; symlink failed: %w\n  Run: sudo ln -s %s %s", binary, err, resolved, dst)
				}
			}
			fmt.Printf("  ✓ Linked %s → %s\n", dst, resolved)
		} else {
			return fmt.Errorf("'%s' not found in PATH or /Applications", binary)
		}
	}
	vsix := filepath.Join(cacheDir, "steer.vsix")
	if err := downloadPluginAsset(vsix, "steer.vsix"); err != nil {
		return err
	}
	fmt.Printf("  Installing via %s...\n", binary)
	if err := exec.Command(binary, "--install-extension", vsix).Run(); err != nil {
		return err
	}

	// Auto-configure kiroCLIPath so the extension can find kiro-cli
	if kiroPath := FindKiroCLI(); kiroPath != "" {
		if err := setIDESetting(binary, "steer.kiroCLIPath", kiroPath); err == nil {
			fmt.Printf("  ✓ Set steer.kiroCLIPath = %s\n", kiroPath)
		}
	}
	return nil
}

// resolveAppBinary finds a CLI binary inside a macOS .app bundle.
func resolveAppBinary(binary string) string {
	appNames := map[string]string{
		"cursor": "/Applications/Cursor.app",
		"code":   "/Applications/Visual Studio Code.app",
	}
	app, ok := appNames[binary]
	if !ok {
		return ""
	}
	candidate := filepath.Join(app, "Contents/Resources/app/bin", binary)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
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

	// Primary: curl from github.com public releases (no auth needed)
	url := fmt.Sprintf("https://github.com/rsanchez-disney/Koda/releases/latest/download/%s", assetName)
	curlBin := "curl"
	if runtime.GOOS == "windows" {
		curlBin = "curl.exe"
	}
	if err := exec.Command(curlBin, "-fsSL", "-o", dest, url).Run(); err == nil {
		return nil
	}

	// Fallback: gh release download
	if ghPath, err := exec.LookPath("gh"); err == nil {
		cmd := exec.Command(ghPath, "release", "download", "--repo", "rsanchez-disney/Koda",
			"--pattern", assetName, "--dir", filepath.Dir(dest), "--clobber")
		cmd.Env = append(os.Environ(), "GH_HOST=github.com")
		if err := cmd.Run(); err == nil {
			downloaded := filepath.Join(filepath.Dir(dest), assetName)
			if downloaded != dest {
				os.Rename(downloaded, dest)
			}
			return nil
		}
	}

	return fmt.Errorf("failed to download %s — check network connection", assetName)
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

// setIDESetting writes a setting into the IDE's user settings.json.
func setIDESetting(binary, key, value string) error {
	home, _ := os.UserHomeDir()
	var settingsPath string
	switch binary {
	case "code":
		switch runtime.GOOS {
		case "darwin":
			settingsPath = filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")
		case "windows":
			settingsPath = filepath.Join(os.Getenv("APPDATA"), "Code", "User", "settings.json")
		default:
			settingsPath = filepath.Join(home, ".config", "Code", "User", "settings.json")
		}
	case "cursor":
		switch runtime.GOOS {
		case "darwin":
			settingsPath = filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json")
		case "windows":
			settingsPath = filepath.Join(os.Getenv("APPDATA"), "Cursor", "User", "settings.json")
		default:
			settingsPath = filepath.Join(home, ".config", "Cursor", "User", "settings.json")
		}
	default:
		return fmt.Errorf("unsupported IDE: %s", binary)
	}

	// Read existing settings or start fresh
	settings := map[string]interface{}{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		// Simple JSON parse (settings.json may have comments, but we do best-effort)
		if err := json.Unmarshal(data, &settings); err != nil {
			// If parse fails, don't overwrite user's file
			return err
		}
	}

	settings[key] = value
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(settingsPath), 0755)
	return os.WriteFile(settingsPath, out, 0644)
}
