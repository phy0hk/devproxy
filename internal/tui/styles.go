package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Header styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	// Status styles
	statusRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")).
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	statusStartingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226"))

	statusStoppingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("208"))

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	statusConfiguredStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	// Process list styles
	processListStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(0, 1)

	selectedProcessStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("229")).
				Bold(true)

	processItemStyle = lipgloss.NewStyle().
				Padding(0, 1)

	// Log panel styles
	logPanelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	logLineStyle = lipgloss.NewStyle()

	stdoutStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	stderrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	lifecycleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))

	proxyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69"))

	// Tab styles
	tabStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("244"))

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("62")).
			Bold(true)

	// Help/styles
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1)

	// Title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Padding(0, 1)

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
)

// GetStatusStyle returns the appropriate style for a process state
func GetStatusStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return statusRunningStyle
	case "stopped", "exited":
		return statusStoppedStyle
	case "starting":
		return statusStartingStyle
	case "stopping":
		return statusStoppingStyle
	case "failed":
		return statusFailedStyle
	case "configured":
		return statusConfiguredStyle
	default:
		return lipgloss.NewStyle()
	}
}
