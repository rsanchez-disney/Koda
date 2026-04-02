package tray

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlantern/systray"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var steerRoot string

// Run launches the menu bar tray app.
func Run(sr string) {
	steerRoot = sr
	systray.Run(onReady, func() {})
}

func onReady() {
	systray.SetTitle("🐾")
	systray.SetTooltip("Koda — Agent Runtime Manager")

	// Status section
	mStatus := systray.AddMenuItem("Loading...", "")
	mStatus.Disable()
	refreshStatus(mStatus)

	systray.AddSeparator()

	// Sync
	mSync := systray.AddMenuItem("⟳ Sync Runtime", "Update steer-runtime")

	// Workspaces submenu
	mWorkspaces := systray.AddMenuItem("Workspaces", "Switch workspace")
	wsItems := refreshWorkspaces(mWorkspaces)

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "")

	// Event loop
	go func() {
		for {
			select {
			case <-mSync.ClickedCh:
				mStatus.SetTitle("⏳ Syncing...")
				target := config.TargetDir("")
				if err := ops.SyncSteerRuntime(steerRoot, target); err != nil {
					mStatus.SetTitle("✗ Sync failed")
				} else {
					refreshStatus(mStatus)
					// Rebuild workspace submenu after sync
					for _, wi := range wsItems {
						wi.item.Hide()
					}
					wsItems = refreshWorkspaces(mWorkspaces)
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
			default:
				for _, wi := range wsItems {
					select {
					case <-wi.item.ClickedCh:
						mStatus.SetTitle("⏳ Applying " + wi.name + "...")
						target := config.TargetDir("")
						ws, err := ops.GetWorkspace(steerRoot, wi.name)
						if err == nil {
							ops.ApplyWorkspace(steerRoot, target, ws)
							refreshStatus(mStatus)
						}
					default:
					}
				}
			}
		}
	}()
}

type wsItem struct {
	name string
	item *systray.MenuItem
}

func refreshStatus(m *systray.MenuItem) {
	settings := config.ReadSteerSettings()
	target := config.TargetDir("")

	var parts []string

	if ver, err := os.ReadFile(filepath.Join(steerRoot, "VERSION")); err == nil {
		parts = append(parts, strings.TrimSpace(string(ver)))
	}
	if settings.ActiveWorkspace != "" {
		parts = append(parts, "ws:"+settings.ActiveWorkspace)
	}
	report := ops.CheckInstallation(steerRoot, target)
	if report.TotalAgents > 0 {
		parts = append(parts, fmt.Sprintf("%d agents", report.TotalAgents))
	}

	if len(parts) > 0 {
		m.SetTitle("🐾 " + strings.Join(parts, " · "))
	} else {
		m.SetTitle("🐾 Koda")
	}
}

func refreshWorkspaces(parent *systray.MenuItem) []wsItem {
	workspaces, _ := ops.ListWorkspaces(steerRoot)
	var items []wsItem
	for _, ws := range workspaces {
		label := ws.Name
		if ws.Description != "" {
			label += " — " + ws.Description
		}
		sub := parent.AddSubMenuItem(label, "Apply "+ws.Name)
		items = append(items, wsItem{name: ws.Name, item: sub})
	}
	return items
}
