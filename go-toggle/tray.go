package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"sync"

	"github.com/getlantern/systray"
)

//go:embed 1.png
var embedPng1 []byte

//go:embed 2.png
var embedPng2 []byte

// ============================================================
// 托盘图标管理
// ============================================================

var (
	iconShown []byte // 显示状态图标（1.png 包装为 .ico）
	iconHidden []byte // 隐藏状态图标（2.png 包装为 .ico）
	iconsLoaded bool
	iconsMu     sync.Mutex
)

// loadIcons 从嵌入的 PNG 数据初始化托盘图标
func loadIcons() {
	iconsMu.Lock()
	defer iconsMu.Unlock()
	if iconsLoaded {
		return
	}

	if len(embedPng1) > 0 {
		iconShown = makeICO(embedPng1)
	}
	if len(embedPng2) > 0 {
		iconHidden = makeICO(embedPng2)
	}
	iconsLoaded = true
}

// makeICO 将 PNG 数据包装成 .ico 文件格式
func makeICO(pngData []byte) []byte {
	var buf bytes.Buffer
	// ICO 头
	binary.Write(&buf, binary.LittleEndian, uint16(0))      // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))      // ICO type
	binary.Write(&buf, binary.LittleEndian, uint16(1))      // 1 image
	// 目录项
	binary.Write(&buf, binary.LittleEndian, uint8(0))       // width (0=256)
	binary.Write(&buf, binary.LittleEndian, uint8(0))       // height (0=256)
	binary.Write(&buf, binary.LittleEndian, uint8(0))       // colors
	binary.Write(&buf, binary.LittleEndian, uint8(0))       // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))      // color planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))     // bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData))) // size
	binary.Write(&buf, binary.LittleEndian, uint32(22))     // offset (6+16)
	buf.Write(pngData)
	return buf.Bytes()
}

// setTrayIconShown 设置托盘图标为显示状态图标
func setTrayIconShown() {
	loadIcons()
	if iconShown != nil {
		systray.SetIcon(iconShown)
	} else {
		// 如果加载失败，用默认图标
		systray.SetIcon(makeFallbackIcon())
	}
}

// setTrayIconHidden 设置托盘图标为隐藏状态图标
func setTrayIconHidden() {
	loadIcons()
	if iconHidden != nil {
		systray.SetIcon(iconHidden)
	} else {
		systray.SetIcon(makeFallbackIcon())
	}
}

// makeFallbackIcon 生成一个简单的黄色方块作为备用图标
func makeFallbackIcon() []byte {
	// 用 makeSmileyIcon 的旧逻辑作为后备
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	yellow := color.RGBA{255, 220, 50, 255}
	black := color.RGBA{0, 0, 0, 255}
	transparent := color.RGBA{0, 0, 0, 0}

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			dx, dy := x-7, y-7
			r := dx*dx + dy*dy
			if r >= 9 && r <= 49 {
				img.Set(x, y, yellow)
			} else {
				img.Set(x, y, transparent)
			}
		}
	}

	// 眼睛
	for y := 3; y <= 4; y++ {
		for x := 3; x <= 4; x++ {
			img.Set(x, y, black)
		}
		for x := 10; x <= 11; x++ {
			img.Set(x, y, black)
		}
	}

	// 嘴巴
	for x := 3; x <= 11; x++ {
		dx2 := x - 7
		yy := 10 + dx2*dx2/8
		img.Set(x, yy, black)
		img.Set(x, yy+1, black)
	}

	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)
	return makeICO(pngBuf.Bytes())
}

func onTrayReady() {
	// 加载图标
	loadIcons()

	// 设置初始图标（显示状态）
	setTrayIconShown()
	systray.SetTitle(productName)
	systray.SetTooltip(productName + " - 左键切换 | 右键退出")

	mToggle := systray.AddMenuItem("切换桌面图标", "切换桌面图标显示/隐藏")
	systray.AddSeparator()
	mSettings := systray.AddMenuItem("设置", "打开设置")
	systray.AddSeparator()
	mExit := systray.AddMenuItem("退出", "退出程序")

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				toggleDesktopIcons()
			case <-mSettings.ClickedCh:
				showSettingsDialog()
			case <-mExit.ClickedCh:
				systrayQuit()
			}
		}
	}()
}

func onTrayExit() {
	showDesktopIcons()
}

func systrayQuit() {
	systray.Quit()
}

// 获取程序版本号
func GetVersion() string {
	return version
}