package apps

import "runtime"

// App defines a catalog entry for a managed application.
type App struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Repo        string     `json:"repo"`
	Platforms   []Platform `json:"platforms"`
	Launch      LaunchCfg  `json:"launch"`
}

// Platform defines an artifact for a specific OS/arch.
type Platform struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Artifact string `json:"artifact"`
}

// LaunchCfg defines how to launch the app per OS.
type LaunchCfg struct {
	Darwin  string `json:"darwin"`
	Windows string `json:"windows"`
}

// Catalog returns all available apps.
func Catalog() []App {
	return []App{
		{
			Name:        "kite",
			Description: "AI-powered desktop companion",
			Repo:        "rsanchez-disney/kite",
			Platforms: []Platform{
				{OS: "darwin", Arch: "arm64", Artifact: "kite-darwin-arm64.tar.gz.enc"},
				{OS: "darwin", Arch: "amd64", Artifact: "kite-darwin-amd64.tar.gz.enc"},
				{OS: "windows", Arch: "amd64", Artifact: "kite-windows-amd64.tar.gz.enc"},
			},
			Launch: LaunchCfg{Darwin: "Kite.app", Windows: "Kite.exe"},
		},
		{
			Name:        "mouseketool",
			Description: "Local AWS companion for backend developers",
			Repo:        "rsanchez-disney/mouseketool",
			Platforms: []Platform{
				{OS: "darwin", Arch: "arm64", Artifact: "mouseketool-darwin-arm64.tar.gz.enc"},
				{OS: "darwin", Arch: "amd64", Artifact: "mouseketool-darwin-amd64.tar.gz.enc"},
				{OS: "windows", Arch: "amd64", Artifact: "mouseketool-windows-amd64.tar.gz.enc"},
			},
			Launch: LaunchCfg{Darwin: "Mouseketool.app", Windows: "Mouseketool.exe"},
		},
	}
}

// Find returns an app by name, or nil if not found.
func Find(name string) *App {
	for _, a := range Catalog() {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

// ResolveArtifact returns the artifact name for the current platform.
func (a *App) ResolveArtifact() string {
	for _, p := range a.Platforms {
		if p.OS == runtime.GOOS && p.Arch == runtime.GOARCH {
			return p.Artifact
		}
	}
	return ""
}

// LaunchCommand returns the launch target for the current OS.
func (a *App) LaunchCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return a.Launch.Darwin
	case "windows":
		return a.Launch.Windows
	default:
		return ""
	}
}
