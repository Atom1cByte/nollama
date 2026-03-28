package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	PrimaryColor      = lipgloss.Color("#7D56F4")
	SecondaryColor    = lipgloss.Color("#B581FD")
	AccentColor       = lipgloss.Color("#F96987")
	WarningColor      = lipgloss.Color("#F98E69")
	SuccessColor      = lipgloss.Color("#2EEDC0")
	WhiteColor        = lipgloss.Color("#FFFFFF")
	GrayColor         = lipgloss.Color("#B0B0B0")
	DeepGrayColor     = lipgloss.Color("#3A3A3A")
	BlackColor        = lipgloss.Color("#000000")
	ShadowColor       = lipgloss.Color("#1A1A1A")
	BorderColor       = lipgloss.Color("#4A4A4A")

	// Styles
	BaseStyle = lipgloss.NewStyle().
		Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		MarginBottom(1)

	ModelNameStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(SecondaryColor)

	ModelSelectedStyle = lipgloss.NewStyle().
		Foreground(WhiteColor).
		Background(PrimaryColor).
		Padding(0, 1)

	SelectedModelItemStyle = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	UnselectedModelItemStyle = lipgloss.NewStyle().
		Foreground(GrayColor)

	HelpStyle = lipgloss.NewStyle().
		Foreground(DeepGrayColor)

	ChatUserStyle = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true)

	ChatBotStyle = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	StatusLabelStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(DeepGrayColor).
		Padding(0, 1)

	SuccessStatusStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(SuccessColor)

	InfoLabelStyle = lipgloss.NewStyle().
		Foreground(GrayColor).
		Italic(true)

	ContainerStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(1, 2).
		BorderForeground(lipgloss.AdaptiveColor{Light: "235", Dark: "240"})

	ChartBoxStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0).
		BorderForeground(AccentColor).
		MarginTop(1)

	MessageContentStyle = lipgloss.NewStyle().
		Foreground(WhiteColor)

	CursorStyle = lipgloss.NewStyle().
		Foreground(AccentColor)

	// Sidebar (Model List)
	SidebarStyle = lipgloss.NewStyle().
		Width(30).
		MarginRight(2).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(PrimaryColor).
		Padding(1)

	MainViewStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(DeepGrayColor).
		Padding(1)
)

func GradientStyle(from, to lipgloss.Color, text string) string {
	return lipgloss.NewStyle().Foreground(from).Render(text)
}
