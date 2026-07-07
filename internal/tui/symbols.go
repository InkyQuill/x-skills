package tui

type symbols struct {
	ProductMark string
	Cursor      string
	Unchecked   string
	Checked     string
	Managed     string
	Unmanaged   string
	Broken      string
	CountPrefix string
	BadgeLeft   string
	BadgeRight  string
}

func symbolsFor(opts Options) symbols {
	if opts.ASCII {
		return symbols{
			ProductMark: "*",
			Cursor:      ">",
			Unchecked:   "[ ]",
			Checked:     "[x]",
			Managed:     "@",
			Unmanaged:   "#",
			Broken:      "!",
			CountPrefix: "x",
			BadgeLeft:   "[",
			BadgeRight:  "]",
		}
	}
	return symbols{
		ProductMark: "◆",
		Cursor:      "›",
		Unchecked:   "□",
		Checked:     "■",
		Managed:     "✓",
		Unmanaged:   "◆",
		Broken:      "▲",
		CountPrefix: "×",
		BadgeLeft:   "",
		BadgeRight:  "",
	}
}

func rootChip(scope, target string) string {
	prefix := "."
	if scope == "global" {
		prefix = "~"
	}
	switch target {
	case "agents":
		return prefix + "Ag"
	case "claude":
		return prefix + "Cl"
	case "codex":
		return prefix + "Cd"
	default:
		return prefix + target
	}
}
