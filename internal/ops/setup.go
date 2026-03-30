package ops

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Dep represents a system dependency.
type Dep struct {
	Name      string
	Binary    string
	Installed bool
	Version   string
	InstallFn func() error
}

// CheckDeps returns the status of all required dependencies.
func CheckDeps() []Dep {
	deps := []Dep{
		{Name: "Node.js", Binary: "node", InstallFn: installNode},
		{Name: "npm", Binary: "npm", InstallFn: installNode},
		{Name: "Git", Binary: "git", InstallFn: installGit},
		{Name: "kiro-cli", Binary: "kiro-cli", InstallFn: installKiroCLI},
		{Name: "GitHub CLI", Binary: "gh", InstallFn: installGH},
	}
	// On Windows, check for WSL
	if runtime.GOOS == "windows" {
		deps = append(deps, Dep{Name: "WSL", Binary: "wsl", InstallFn: installWSL})
	}

	for i := range deps {
		path, err := exec.LookPath(deps[i].Binary)
		if err == nil {
			deps[i].Installed = true
			out, _ := exec.Command(path, "--version").Output()
			deps[i].Version = strings.TrimSpace(string(out))
		}
	}
	return deps
}

// InstallDep attempts to install a dependency.
func InstallDep(d Dep) error {
	if d.InstallFn == nil {
		return fmt.Errorf("no installer for %s", d.Name)
	}
	return d.InstallFn()
}

func hasBrew() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

func hasWinget() bool {
	_, err := exec.LookPath("winget")
	return err == nil
}

func hasApt() bool {
	_, err := exec.LookPath("apt-get")
	return err == nil
}

func hasChoco() bool {
	_, err := exec.LookPath("choco")
	return err == nil
}

func runVisible(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	fmt.Printf("  \u25b8 %s %s\n", name, strings.Join(args, " "))
	return cmd.Run()
}

func installNode() error {
	switch runtime.GOOS {
	case "darwin":
		if hasBrew() {
			return runVisible("brew", "install", "node")
		}
		fmt.Println("  Install Homebrew first: https://brew.sh")
		fmt.Println("  Then: brew install node")
	case "linux":
		if hasApt() {
			if err := runVisible("sudo", "apt-get", "update", "-qq"); err != nil {
				return err
			}
			return runVisible("sudo", "apt-get", "install", "-y", "nodejs", "npm")
		}
		if hasBrew() {
			return runVisible("brew", "install", "node")
		}
		fmt.Println("  Install via: https://nodejs.org/en/download")
	case "windows":
		if hasWinget() {
			return runVisible("winget", "install", "OpenJS.NodeJS.LTS")
		}
		if hasChoco() {
			return runVisible("choco", "install", "nodejs-lts", "-y")
		}
		fmt.Println("  Install via: https://nodejs.org/en/download")
	}
	return nil
}

func installGit() error {
	switch runtime.GOOS {
	case "darwin":
		if hasBrew() {
			return runVisible("brew", "install", "git")
		}
		return runVisible("xcode-select", "--install")
	case "linux":
		if hasApt() {
			return runVisible("sudo", "apt-get", "install", "-y", "git")
		}
		if hasBrew() {
			return runVisible("brew", "install", "git")
		}
		fmt.Println("  Install via: https://git-scm.com/downloads")
	case "windows":
		if hasWinget() {
			return runVisible("winget", "install", "Git.Git")
		}
		if hasChoco() {
			return runVisible("choco", "install", "git", "-y")
		}
		fmt.Println("  Install via: https://git-scm.com/downloads")
	}
	return nil
}

func installKiroCLI() error {
	fmt.Println("  Install kiro-cli:")
	fmt.Println("    npm install -g @anthropic/kiro-cli")
	fmt.Println("  Or download from your internal distribution channel.")
	if _, err := exec.LookPath("npm"); err == nil {
		fmt.Print("  Attempt npm install now? This may require sudo. [y/N]: ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(answer) == "y" {
			return runVisible("npm", "install", "-g", "@anthropic/kiro-cli")
		}
	}
	return nil
}

func installGH() error {
	switch runtime.GOOS {
	case "darwin":
		if hasBrew() {
			return runVisible("brew", "install", "gh")
		}
		fmt.Println("  Install Homebrew first, then: brew install gh")
	case "linux":
		if hasBrew() {
			return runVisible("brew", "install", "gh")
		}
		fmt.Println("  Install via: https://github.com/cli/cli/blob/trunk/docs/install_linux.md")
	case "windows":
		if hasWinget() {
			return runVisible("winget", "install", "GitHub.cli")
		}
		if hasChoco() {
			return runVisible("choco", "install", "gh", "-y")
		}
		fmt.Println("  Install via: https://cli.github.com")
	}
	return nil
}

func hasWSLDistro() bool {
	out, err := exec.Command("wsl", "--list", "--quiet").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func installWSL() error {
	// Check if wsl command exists but no distro installed
	if _, err := exec.LookPath("wsl"); err != nil {
		fmt.Println("  WSL is not available on this system.")
		fmt.Println("  Install via: wsl --install")
		fmt.Println("  Or enable it in Windows Features > Windows Subsystem for Linux")
		fmt.Print("  Attempt wsl --install now? [y/N]: ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(answer) == "y" {
			return runVisible("wsl", "--install")
		}
		return nil
	}

	if !hasWSLDistro() {
		fmt.Println("  WSL is installed but no Linux distro found.")
		fmt.Print("  Install Ubuntu? [y/N]: ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(answer) == "y" {
			if err := runVisible("wsl", "--install", "-d", "Ubuntu"); err != nil {
				return err
			}
		}
	}

	// Install koda inside WSL
	fmt.Println("  Installing koda inside WSL...")
	return runVisible("wsl", "--", "bash", "-c",
		"curl -fsSL https://github.disney.com/raw/SANCR225/steer-runtime/main/tools/install-koda.sh | bash")
}
