package theme

import "github.com/charmbracelet/lipgloss"

// Theme encapsulates the visual palette for the CRM UI.
type Theme struct {
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Accent    lipgloss.Style
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
	Danger    lipgloss.Style
	Faint     lipgloss.Style
	Highlight lipgloss.Style
	Border    lipgloss.Style
	HelpKey   lipgloss.Style
	HelpValue lipgloss.Style
}

// Default returns a high-contrast palette that plays nicely with common terminals.
func Default() Theme {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	return Theme{
		Title:     lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true).Underline(true),
		Subtitle:  lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Bold(true),
		Accent:    lipgloss.NewStyle().Foreground(lipgloss.Color("219")).Bold(true),
		Primary:   base.Copy().Foreground(lipgloss.Color("81")),
		Secondary: lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
		Success:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		Warning:   lipgloss.NewStyle().Foreground(lipgloss.Color("227")).Bold(true),
		Danger:    lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		Faint:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		Highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true),
		Border:    lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		HelpKey:   lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true),
		HelpValue: lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	}
}
