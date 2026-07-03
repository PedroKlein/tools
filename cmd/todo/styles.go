package main

import "github.com/charmbracelet/lipgloss"

// Osaka Jade color palette.
// Source: configs-shared/.pi/agent/themes/osaka-jade.json
var (
	ColorAccent = lipgloss.Color("#549E6A")
	ColorBright = lipgloss.Color("#63B07A")
	ColorFg     = lipgloss.Color("#C1C497")
	ColorMuted  = lipgloss.Color("#627A6C")
	ColorDim    = lipgloss.Color("#435F50")
	ColorWhite  = lipgloss.Color("#F6F5DD")
	ColorRed    = lipgloss.Color("#FF5345")
	ColorYellow = lipgloss.Color("#E5C736")
	ColorCyan   = lipgloss.Color("#2DD5B7")
	ColorBlue   = lipgloss.Color("#7AA2F7")
	ColorPurple = lipgloss.Color("#BB9AF7")
	ColorJade   = lipgloss.Color("#459451")
	ColorTeal   = lipgloss.Color("#509475")
)

// Layout styles — no background, let terminal handle it.
var (
	StyleApp = lipgloss.NewStyle()

	StyleContent = lipgloss.NewStyle().
			Padding(0, 1)

	StyleHelpBar = lipgloss.NewStyle().
			Foreground(ColorDim).
			Padding(0, 1)
)

// Base styles.
var (
	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorBright).
			Bold(true)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleAccent = lipgloss.NewStyle().
			Foreground(ColorAccent)

	StyleFg = lipgloss.NewStyle().
		Foreground(ColorFg)

	StyleDim = lipgloss.NewStyle().
			Foreground(ColorDim)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorRed)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorBright)
)

// Tab styles.
var (
	StyleTabActive = lipgloss.NewStyle().
			Foreground(ColorBright).
			Bold(true).
			Padding(0, 1)

	StyleTabInactive = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)

	StyleTabBar = lipgloss.NewStyle().
			Padding(0, 1).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorDim)
)

// Task styles.
var (
	StyleTaskSelected = lipgloss.NewStyle().
				Foreground(ColorWhite).
				Bold(true)

	StyleTaskNormal = lipgloss.NewStyle().
			Foreground(ColorFg)

	StyleTaskDone = lipgloss.NewStyle().
			Foreground(ColorDim).
			Strikethrough(true)

	StyleTaskBlocked = lipgloss.NewStyle().
				Foreground(ColorYellow)
)

// Priority badge styles.
var (
	StylePriorityHigh = lipgloss.NewStyle().
				Foreground(ColorRed).
				Bold(true)

	StylePriorityMedium = lipgloss.NewStyle().
				Foreground(ColorYellow)

	StylePriorityLow = lipgloss.NewStyle().
				Foreground(ColorDim)
)

// Type badge styles.
var (
	StyleTypeFeature  = lipgloss.NewStyle().Foreground(ColorBlue)
	StyleTypeBug      = lipgloss.NewStyle().Foreground(ColorRed)
	StyleTypeChore    = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleTypeResearch = lipgloss.NewStyle().Foreground(ColorPurple)
	StyleTypeReview   = lipgloss.NewStyle().Foreground(ColorCyan)
	StyleTypePersonal = lipgloss.NewStyle().Foreground(ColorAccent)
)

// Help bar.
var (
	StyleHelp = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleHelpKey = lipgloss.NewStyle().
			Foreground(ColorAccent)
)

// Form styles.
var (
	StyleFormBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorTeal).
			Padding(1, 2)

	StyleFormLabel = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(14)

	StyleFormValue = lipgloss.NewStyle().
			Foreground(ColorFg)

	StyleFormActive = lipgloss.NewStyle().
			Foreground(ColorBright).
			Bold(true)

	StyleButton = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true).
			Padding(0, 2)

	StyleButtonInactive = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 2)
)
