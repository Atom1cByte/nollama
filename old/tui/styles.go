package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#B581FD")
	AccentColor    = lipgloss.Color("#F96987")
	WarningColor   = lipgloss.Color("#F98E69")
	SuccessColor   = lipgloss.Color("#2EEDC0")
	WhiteColor     = lipgloss.Color("#FFFFFF")
	GrayColor      = lipgloss.Color("#B0B0B0")
	DeepGrayColor  = lipgloss.Color("#3A3A3A")
	BlackColor     = lipgloss.Color("#000000")
	ShadowColor    = lipgloss.Color("#1A1A1A")
	BorderColor    = lipgloss.Color("#4A4A4A")

	BaseStyle = lipgloss.NewStyle().
			Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			MarginBottom(1)

	FancyHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			Padding(0, 1).
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(PrimaryColor).
			BorderBottom(true)

	FancyContainer = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2)

	SubtleStyle = lipgloss.NewStyle().
			Foreground(GrayColor).
			Italic(true)

	AccentStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(AccentColor).
			Padding(1, 2)

	PromptStyle = lipgloss.NewStyle().
			Foreground(GrayColor)

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
			Foreground(DeepGrayColor).
			MarginTop(1)

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
