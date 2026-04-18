package ui

type layoutMode int

const (
	layoutWide   layoutMode = iota // >= 100 cols: sidebar + multi-col grid
	layoutMedium                   // 80-99:     sidebar + grid
	layoutNarrow                   // < 80:      grid only, sidebar via overlay
)

const (
	sidebarWidth = 22
	sidebarGap   = 1
)

func modeFor(width int) layoutMode {
	switch {
	case width >= 100:
		return layoutWide
	case width >= 80:
		return layoutMedium
	default:
		return layoutNarrow
	}
}
