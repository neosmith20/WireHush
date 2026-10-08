/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package ui

import (
	"fmt"
	"log"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/lxn/walk"
	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/l18n"
	"golang.zx2c4.com/wireguard/windows/manager"
)

var (
	noTrayAvailable              = false
	shouldQuitManagerWhenExiting = false
	startTime                    = time.Now()
	IsAdmin                      = false // A global, because this really is global for the process
)

func RunUI() {
	log.Printf("RunUI ENTER")
	runtime.LockOSThread()
	windows.SetProcessPriorityBoost(windows.CurrentProcess(), false)
	defer func() {
		if err := recover(); err != nil {
			showErrorCustom(nil, "Panic", fmt.Sprint(err, "\n\n", string(debug.Stack())))
			panic(err)
		}
	}()

	var (
		err  error
		mtw  *ManageTunnelsWindow
		tray *Tray
	)

	for mtw == nil {
		log.Printf("RunUI before NewManageTunnelsWindow")
		mtw, err = NewManageTunnelsWindow()
		if err != nil {
			log.Printf("RunUI NewManageTunnelsWindow error: %v", err)
			time.Sleep(time.Millisecond * 400)
		} else {
			log.Printf("RunUI after NewManageTunnelsWindow success")
		}
	}

	for tray == nil {
		log.Printf("RunUI before NewTray")
		tray, err = NewTray(mtw)
		if err != nil {
			log.Printf("RunUI NewTray failure: %v", err)
			// A missing or unavailable notification area must never prevent the
			// main manager UI from starting. This is common in remote, kiosk, and
			// shell-restart sessions, and the former retry loop kept the UI child
			// alive forever before it ever entered its message loop.
			noTrayAvailable = true
			break
		}
		log.Printf("RunUI after NewTray success")
	}

	manager.IPCClientRegisterManagerStopping(func() {
		mtw.Synchronize(func() {
			walk.App().Exit(0)
		})
	})

	onUpdateNotification := func(updateState manager.UpdateState) {
		if updateState == manager.UpdateStateUnknown {
			return
		}
		mtw.Synchronize(func() {
			switch updateState {
			case manager.UpdateStateFoundUpdate:
				mtw.UpdateFound()
				if tray != nil && IsAdmin {
					tray.UpdateFound()
				}
			case manager.UpdateStateUpdatesDisabledUnofficialBuild:
				mtw.SetTitle(l18n.Sprintf("%s (unsigned build, no updates)", mtw.Title()))
			}
		})
	}
	manager.IPCClientRegisterUpdateFound(onUpdateNotification)
	go func() {
		updateState, err := manager.IPCClientUpdateState()
		if err == nil {
			onUpdateNotification(updateState)
		}
	}()

	if tray == nil {
		log.Printf("RunUI before mtw.Show")
		// Use Walk's form lifecycle rather than a raw ShowWindow call. Show
		// restores persistent bounds and publishes visibility to the layout;
		// calling the raw API before Run left the completed form invisible on
		// this VM.
		mtw.Show()
		log.Printf("RunUI after mtw.Show")
		log.Printf("RunUI before raise")
		raise(mtw.Handle())
		log.Printf("RunUI after raise")
	}

	log.Printf("RunUI immediately before mtw.Run")
	mtw.Run()
	log.Printf("RunUI immediately after mtw.Run returned")
	if tray != nil {
		tray.Dispose()
	}
	mtw.Dispose()

	if shouldQuitManagerWhenExiting {
		_, err := manager.IPCClientQuit(true)
		if err != nil {
			showErrorCustom(nil, l18n.Sprintf("Error Exiting WireHush"), l18n.Sprintf("Unable to exit service due to: %v. You may want to stop WireHush from the service manager.", err))
		}
	}
}

func onQuit() {
	shouldQuitManagerWhenExiting = true
	walk.App().Exit(0)
}

func showError(err error, owner walk.Form) bool {
	if err == nil {
		return false
	}

	showErrorCustom(owner, l18n.Sprintf("Error"), err.Error())

	return true
}

func showErrorCustom(owner walk.Form, title, message string) {
	walk.MsgBox(owner, title, message, walk.MsgBoxIconError)
}

func showWarningCustom(owner walk.Form, title, message string) {
	walk.MsgBox(owner, title, message, walk.MsgBoxIconWarning)
}
