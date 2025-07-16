package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	PrimaryColor   = lipgloss.Color("#FF6B6B")
	SecondaryColor = lipgloss.Color("#4ECDC4")
	AccentColor    = lipgloss.Color("#45B7D1")
	SuccessColor   = lipgloss.Color("#96CEB4")
	WarningColor   = lipgloss.Color("#FFEAA7")
	ErrorColor     = lipgloss.Color("#DDA0DD")
	InfoColor      = lipgloss.Color("#74B9FF")

	// Text styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Padding(0, 1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	DescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)

	// Status styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(InfoColor).
			Bold(true)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(AccentColor).
			Padding(1, 2)

	HighlightStyle = lipgloss.NewStyle().
			Background(AccentColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	// Table styles
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true).
				Align(lipgloss.Center)

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Progress styles
	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(SuccessColor)

	ProgressTrackStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#333333"))
)

// Helpers
func RenderSuccess(text string) string {
	return SuccessStyle.Render("[V] " + text)
}

func RenderError(text string) string {
	return ErrorStyle.Render("[X] " + text)
}

func RenderWarning(text string) string {
	return WarningStyle.Render("[W]  " + text)
}

func RenderInfo(text string) string {
	return InfoStyle.Render("[I]  " + text)
}

func RenderBox(title, content string) string {
	return BoxStyle.Render(
		TitleStyle.Render(title) + "\n" + content,
	)
}
