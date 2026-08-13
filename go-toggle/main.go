package main

import (
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/getlantern/systray"
)

var (
	cfg *Config
	version = "v2.1"
	productName = "自动隐藏桌面图标"
)

func main() {
	// 关闭日志输出（不写日志文件，不打印到控制台）
	log.SetOutput(io.Discard)

	// 加载配置
	cfg = loadConfig()

	// 显示桌面图标（确保初始状态）
	showDesktopIcons()

	// 启动钩子线程（鼠标/键盘/前台窗口事件）
	startHookThread()

	// 运行系统托盘（阻塞主线程）
	systray.Run(onTrayReady, onTrayExit)
}

// 用于接收退出信号的通道
func waitForExit() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	postQuit()
}

