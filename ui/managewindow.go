/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package ui

import (
	"sync"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/l18n"
	"golang.zx2c4.com/wireguard/windows/manager"
	"golang.zx2c4.com/wireguard/windows/product"
)

type ManageTunnelsWindow struct {
	walk.FormBase

	pageHost     *walk.Composite
	header       *productHeader
	tunnelsPage  *TunnelsPage
	logPage      *LogPage
	settingsPage *SettingsPage
	updatePage   *UpdatePage

	tunnelChangedCB *manager.TunnelChangeCallback
}

type productHeader struct {
	connectionsButton *darkButton
	logButton         *darkButton
	settingsButton    *darkButton
}

const (
	manageWindowWindowClass = product.ManagerWindowClass
	raiseMsg                = win.WM_USER + 0x3510
	aboutWireHushCmd        = 0x37
)

var taskbarButtonCreatedMsg uint32

var initedManageTunnels sync.Once

func NewManageTunnelsWindow() (*ManageTunnelsWindow, error) {
	initedManageTunnels.Do(func() {
		walk.AppendToWalkInit(func() {
			walk.MustRegisterWindowClass(manageWindowWindowClass)
			taskbarButtonCreatedMsg = win.RegisterWindowMessage(windows.StringToUTF16Ptr("TaskbarButtonCreated"))
		})
	})

	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	font, err := walk.NewFont("Segoe UI", 9, 0)
	if err != nil {
		return nil, err
	}

	mtw := new(ManageTunnelsWindow)
	mtw.SetName(product.Name)

	err = walk.InitWindow(mtw, nil, manageWindowWindowClass, win.WS_OVERLAPPEDWINDOW, win.WS_EX_CONTROLPARENT)
	if err != nil {
		return nil, err
	}
	disposables.Add(mtw)
	win.ChangeWindowMessageFilterEx(mtw.Handle(), raiseMsg, win.MSGFLT_ALLOW, nil)
	mtw.SetPersistent(true)

	if icon, err := loadLogoIcon(32); err == nil {
		mtw.SetIcon(icon)
	}
	mtw.SetTitle(product.ManagerWindowTitle)
	mtw.SetFont(font)
	mtw.SetSize(walk.Size{1400, 900})
	mtw.SetMinMaxSize(walk.Size{1100, 700}, walk.Size{0, 0})
	applyDarkWindow(mtw.Handle())
	mtw.SetBackground(uiCanvasBrush)
	vlayout := walk.NewVBoxLayout()
	vlayout.SetMargins(walk.Margins{})
	vlayout.SetSpacing(0)
	mtw.SetLayout(vlayout)
	if mtw.header, err = addProductHeader(mtw, &disposables); err != nil {
		return nil, err
	}
	mtw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		// "Close to tray" instead of exiting application
		*canceled = true
		if !noTrayAvailable {
			mtw.Hide()
		} else {
			win.ShowWindow(mtw.Handle(), win.SW_MINIMIZE)
		}
	})

	if mtw.pageHost, err = walk.NewComposite(mtw); err != nil {
		return nil, err
	}
	pageLayout := walk.NewVBoxLayout()
	pageLayout.SetMargins(walk.Margins{})
	mtw.pageHost.SetLayout(pageLayout)
	applyDarkSurface(mtw.pageHost, uiCanvasBrush)

	if mtw.tunnelsPage, err = NewTunnelsPage(mtw.pageHost); err != nil {
		return nil, err
	}
	mtw.tunnelsPage.CreateToolbar()

	if mtw.logPage, err = NewLogPage(mtw.pageHost); err != nil {
		return nil, err
	}

	if mtw.settingsPage, err = NewSettingsPage(mtw.pageHost); err != nil {
		return nil, err
	}
	mtw.logPage.SetVisible(false)
	mtw.settingsPage.SetVisible(false)
	mtw.header.connectionsButton.Clicked().Attach(func() { mtw.showPage(mtw.tunnelsPage.Composite) })
	mtw.header.logButton.Clicked().Attach(func() {
		mtw.showPage(mtw.logPage.Composite)
	})
	mtw.header.settingsButton.Clicked().Attach(func() {
		mtw.showPage(mtw.settingsPage.Composite)
	})

	mtw.VisibleChanged().Attach(func() {
		if mtw.Visible() {
			mtw.tunnelsPage.updateConfView()
			win.SetForegroundWindow(mtw.Handle())
			win.BringWindowToTop(mtw.Handle())
			mtw.logPage.scrollToBottom()
		}
	})

	mtw.tunnelChangedCB = manager.IPCClientRegisterTunnelChange(mtw.onTunnelChange)
	globalState, _ := manager.IPCClientGlobalState()
	mtw.onTunnelChange(nil, manager.TunnelUnknown, globalState, nil)

	systemMenu := win.GetSystemMenu(mtw.Handle(), false)
	if systemMenu != 0 {
		win.InsertMenuItem(systemMenu, 0, true, &win.MENUITEMINFO{
			CbSize:     uint32(unsafe.Sizeof(win.MENUITEMINFO{})),
			FMask:      win.MIIM_ID | win.MIIM_STRING | win.MIIM_FTYPE,
			FType:      win.MIIM_STRING,
			DwTypeData: windows.StringToUTF16Ptr(l18n.Sprintf("&About WireHush…")),
			WID:        uint32(aboutWireHushCmd),
		})
		win.InsertMenuItem(systemMenu, 1, true, &win.MENUITEMINFO{
			CbSize: uint32(unsafe.Sizeof(win.MENUITEMINFO{})),
			FMask:  win.MIIM_TYPE,
			FType:  win.MFT_SEPARATOR,
		})
	}

	disposables.Spare()

	return mtw, nil
}

func addProductHeader(parent walk.Container, disposables *walk.Disposables) (*productHeader, error) {
	productHeader := new(productHeader)
	header, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	headerLayout := walk.NewHBoxLayout()
	headerLayout.SetMargins(walk.Margins{20, 16, 20, 14})
	header.SetLayout(headerLayout)
	header.SetMinMaxSize(walk.Size{0, 92}, walk.Size{0, 92})
	applyDarkSurface(header, uiHeaderBrush)

	icon, err := loadLogoIcon(40)
	if err == nil {
		imageView, imageErr := walk.NewImageView(header)
		if imageErr != nil {
			return nil, imageErr
		}
		imageView.SetMode(walk.ImageViewModeCenter)
		imageView.SetMinMaxSize(walk.Size{52, 52}, walk.Size{52, 52})
		if err := imageView.SetImage(icon); err != nil {
			return nil, err
		}
	}

	labels, err := walk.NewComposite(header)
	if err != nil {
		return nil, err
	}
	labelsLayout := walk.NewVBoxLayout()
	labelsLayout.SetMargins(walk.Margins{10, 0, 0, 0})
	labelsLayout.SetSpacing(0)
	labels.SetLayout(labelsLayout)
	title, err := walk.NewLabel(labels)
	if err != nil {
		return nil, err
	}
	title.SetText(l18n.Sprintf("WireHush"))
	title.SetTextColor(uiTextColor)
	titleFont, err := walk.NewFont("Segoe UI Semibold", 22, 0)
	if err == nil {
		title.SetFont(titleFont)
		disposables.Add(titleFont)
	}
	subtitle, err := walk.NewLabel(labels)
	if err != nil {
		return nil, err
	}
	subtitle.SetText(l18n.Sprintf("Private network control"))
	applyMutedText(subtitle)
	walk.NewHSpacer(header)
	if productHeader.connectionsButton, err = newDarkButton(header, l18n.Sprintf("Connections"), false); err != nil {
		return nil, err
	}
	if productHeader.logButton, err = newDarkButton(header, l18n.Sprintf("Log"), false); err != nil {
		return nil, err
	}
	if productHeader.settingsButton, err = newDarkButton(header, l18n.Sprintf("Settings"), false); err != nil {
		return nil, err
	}

	accent, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	accent.SetMinMaxSize(walk.Size{0, 3}, walk.Size{0, 3})
	brush, err := walk.NewSolidColorBrush(walk.RGB(23, 195, 210))
	if err != nil {
		return nil, err
	}
	accent.SetBackground(brush)
	disposables.Add(brush)
	return productHeader, nil
}

func (mtw *ManageTunnelsWindow) Dispose() {
	if mtw.tunnelChangedCB != nil {
		mtw.tunnelChangedCB.Unregister()
		mtw.tunnelChangedCB = nil
	}
	mtw.FormBase.Dispose()
}

func (mtw *ManageTunnelsWindow) updateProgressIndicator(globalState manager.TunnelState) {
	pi := mtw.ProgressIndicator()
	if pi == nil {
		return
	}
	switch globalState {
	case manager.TunnelStopping, manager.TunnelStarting:
		pi.SetState(walk.PIIndeterminate)
	default:
		pi.SetState(walk.PINoProgress)
	}
	if icon, err := iconForState(globalState, 16); err == nil {
		if globalState == manager.TunnelStopped {
			icon = nil
		}
		pi.SetOverlayIcon(icon, textForState(globalState, false))
	}
}

func (mtw *ManageTunnelsWindow) onTunnelChange(tunnel *manager.Tunnel, state, globalState manager.TunnelState, err error) {
	mtw.Synchronize(func() {
		mtw.updateProgressIndicator(globalState)

		if err != nil && mtw.Visible() {
			errMsg := err.Error()
			if len(errMsg) > 0 && errMsg[len(errMsg)-1] != '.' {
				errMsg += "."
			}
			showWarningCustom(mtw, l18n.Sprintf("Tunnel Error"), l18n.Sprintf("%s\n\nPlease consult the log for more information.", errMsg))
		}
	})
}

func (mtw *ManageTunnelsWindow) UpdateFound() {
	if mtw.updatePage != nil {
		return
	}
	if IsAdmin {
		mtw.SetTitle(l18n.Sprintf("%s (out of date)", mtw.Title()))
	}
	// Update presentation remains available from the tray notification. The
	// dashboard shell deliberately has no stock tab strip to host a page here.
}

func (mtw *ManageTunnelsWindow) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_QUERYENDSESSION:
		if lParam == win.ENDSESSION_CLOSEAPP {
			return win.TRUE
		}
	case win.WM_ENDSESSION:
		if lParam == win.ENDSESSION_CLOSEAPP && wParam == 1 {
			walk.App().Exit(198)
		}
	case win.WM_SYSCOMMAND:
		if wParam == aboutWireHushCmd {
			onAbout(mtw)
			return 0
		}
	case raiseMsg:
		if mtw.tunnelsPage == nil || mtw.pageHost == nil {
			mtw.Synchronize(func() {
				mtw.SendMessage(msg, wParam, lParam)
			})
			return 0
		}
		if !mtw.Visible() {
			mtw.tunnelsPage.listView.SelectFirstActiveTunnel()
			mtw.showPage(mtw.tunnelsPage.Composite)
		}
		if mtw.updatePage != nil {
			mtw.showPage(mtw.tunnelsPage.Composite)
		}
		raise(mtw.Handle())
		return 0
	case taskbarButtonCreatedMsg:
		ret := mtw.FormBase.WndProc(hwnd, msg, wParam, lParam)
		go func() {
			globalState, err := manager.IPCClientGlobalState()
			if err == nil {
				mtw.Synchronize(func() {
					mtw.updateProgressIndicator(globalState)
				})
			}
		}()
		return ret
	}

	return mtw.FormBase.WndProc(hwnd, msg, wParam, lParam)
}

func (mtw *ManageTunnelsWindow) showPage(page *walk.Composite) {
	mtw.tunnelsPage.SetVisible(page == mtw.tunnelsPage.Composite)
	mtw.logPage.SetVisible(page == mtw.logPage.Composite)
	mtw.settingsPage.SetVisible(page == mtw.settingsPage.Composite)
}
