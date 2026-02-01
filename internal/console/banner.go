package console

import (
	"fmt"

	"kctl/config"
	"kctl/internal/session"
)

// Banner ASCII Art
const banner = `
  ██╗  ██╗ ██████╗████████╗██╗     
  ██║ ██╔╝██╔════╝╚══██╔══╝██║     
  █████╔╝ ██║        ██║   ██║     
  ██╔═██╗ ██║        ██║   ██║     
  ██║  ██╗╚██████╗   ██║   ███████╗
  ╚═╝  ╚═╝ ╚═════╝   ╚═╝   ╚══════╝
`

// Version 版本号
const Version = "v1.0.0"

// PrintBanner 打印 Banner
func PrintBanner(s *session.Session) {
	p := s.Printer

	// 打印 ASCII Art
	p.PrintColored(config.ColorCyan, banner)
	p.Println()

	// 打印版本和信息
	p.Printf("  %s %s\n",
		p.Colored(config.ColorWhite, "Kubelet Security Assessment Tool"),
		p.Colored(config.ColorGray, Version))
	p.Println()

	// 打印运行模式
	p.Printf("  %s Mode: %s\n",
		p.Colored(config.ColorBlue, "[*]"),
		p.Colored(config.ColorGreen, s.GetModeString()))

	// 打印目标信息
	if s.Config.KubeletIP != "" {
		targetInfo := fmt.Sprintf("%s:%d", s.Config.KubeletIP, s.Config.KubeletPort)
		note := ""
		if s.InPod {
			note = " (auto-detected)"
		}
		p.Printf("  %s Target: %s%s\n",
			p.Colored(config.ColorBlue, "[*]"),
			p.Colored(config.ColorYellow, targetInfo),
			p.Colored(config.ColorGray, note))
	} else {
		p.Printf("  %s Target: %s\n",
			p.Colored(config.ColorBlue, "[*]"),
			p.Colored(config.ColorGray, "(not set, use 'set target <ip>')"))
	}

	// 打印帮助提示
	p.Printf("  %s Type '%s' for available commands\n",
		p.Colored(config.ColorBlue, "[*]"),
		p.Colored(config.ColorGreen, "help"))
	p.Println()
}
