package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var bannerGradient = []lipgloss.Style{
	lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#22D3EE"}),
	lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#38BDF8"}),
	lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#818CF8"}),
	lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#8B5CF6", Dark: "#A78BFA"}),
	lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#818CF8"}),
	lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#38BDF8"}),
}

var bannerLines = []string{
	"   ██╗  ██╗ ██████╗ ██████╗  █████╗",
	"   ██║ ██╔╝██╔═══██╗██╔══██╗██╔══██╗",
	"   █████╔╝ ██║   ██║██║  ██║███████║",
	"   ██╔═██╗ ██║   ██║██║  ██║██╔══██║",
	"   ██║  ██╗╚██████╔╝██████╔╝██║  ██║",
	"   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝  ╚═╝",
}

func PrintBanner(version string) {
	for i, line := range bannerLines {
		fmt.Println(bannerGradient[i].Render(line))
	}
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}).Render(
		fmt.Sprintf("   Agent Runtime Manager  v%s", version))
	fmt.Println(subtitle)
	fmt.Println()
}
