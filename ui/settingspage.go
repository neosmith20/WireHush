//go:build windows

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

package ui

import (
	"fmt"

	"github.com/lxn/walk"

	"golang.zx2c4.com/wireguard/windows/bootstrap"
	"golang.zx2c4.com/wireguard/windows/l18n"
	"golang.zx2c4.com/wireguard/windows/manager"
)

type SettingsPage struct {
	*walk.Composite
	settings bootstrap.Settings
	model    *bootstrapSettingsModel
	list     *walk.ListBox
	status   *walk.TextLabel
}

func NewSettingsPage(parent walk.Container) (*SettingsPage, error) {
	page := &SettingsPage{}
	var err error
	page.settings, err = manager.IPCClientBootstrapSettings()
	if err != nil {
		return nil, err
	}

	var disposables walk.Disposables
	defer disposables.Treat()
	if page.Composite, err = walk.NewComposite(parent); err != nil {
		return nil, err
	}
	disposables.Add(page)
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{18, 18, 18, 18})
	layout.SetSpacing(10)
	page.SetLayout(layout)
	applyDarkSurface(page, uiCanvasBrush)

	intro, err := walk.NewTextLabel(page)
	if err != nil {
		return nil, err
	}
	intro.SetText(l18n.Sprintf("Bootstrap DNS resolvers are used only to locate a configured DNS-over-HTTPS endpoint. Changes apply when the next tunnel connects."))
	intro.SetTextAlignment(walk.AlignHNearVCenter)
	intro.SetTextColor(uiTextColor)

	page.model = &bootstrapSettingsModel{entries: page.settings.Entries}
	if page.list, err = walk.NewListBox(page); err != nil {
		return nil, err
	}
	applyDarkWindow(page.list.Handle())
	page.list.SetBackground(uiCardBrush)
	if err := page.list.SetModel(page.model); err != nil {
		return nil, err
	}
	page.list.CurrentIndexChanged().Attach(func() { page.updateButtons() })
	page.list.ItemActivated().Attach(func() { page.toggleSelected() })

	buttons, err := walk.NewComposite(page)
	if err != nil {
		return nil, err
	}
	buttons.SetLayout(walk.NewHBoxLayout())
	buttons.Layout().SetMargins(walk.Margins{})
	for _, item := range []struct {
		text string
		fn   func()
	}{
		{l18n.Sprintf("Enable / Disable"), page.toggleSelected},
		{l18n.Sprintf("Move Up"), func() { page.moveSelected(-1) }},
		{l18n.Sprintf("Move Down"), func() { page.moveSelected(1) }},
		{l18n.Sprintf("Add…"), page.addCustom},
		{l18n.Sprintf("Remove"), page.removeSelected},
		{l18n.Sprintf("Restore Defaults"), page.restoreDefaults},
	} {
		button, err := walk.NewPushButton(buttons)
		if err != nil {
			return nil, err
		}
		button.SetText(item.text)
		button.SetBackground(uiCardBrush)
		button.Clicked().Attach(item.fn)
	}

	page.status, err = walk.NewTextLabel(page)
	if err != nil {
		return nil, err
	}
	page.status.SetText("")
	applyMutedText(page.status)
	page.updateButtons()
	disposables.Spare()
	return page, nil
}

func (page *SettingsPage) updateButtons() {
	if page.status == nil || page.list == nil {
		return
	}
	index := page.list.CurrentIndex()
	if index >= 0 && index < len(page.settings.Entries) {
		entry := page.settings.Entries[index]
		kind := l18n.Sprintf("built-in")
		if entry.Custom {
			kind = l18n.Sprintf("custom")
		}
		state := l18n.Sprintf("disabled")
		if entry.Enabled {
			state = l18n.Sprintf("enabled")
		}
		page.status.SetText(l18n.Sprintf("%s is %s (%s).", entry.Address, state, kind))
	} else {
		page.status.SetText(l18n.Sprintf("Select a resolver to change its order or state."))
	}
}

func (page *SettingsPage) persist() bool {
	if err := manager.IPCClientSaveBootstrapSettings(page.settings); err != nil {
		page.status.SetText(l18n.Sprintf("Unable to save bootstrap DNS settings: %v", err))
		return false
	}
	page.model.entries = page.settings.Entries
	page.model.PublishItemsReset()
	page.updateButtons()
	return true
}

func (page *SettingsPage) toggleSelected() {
	index := page.list.CurrentIndex()
	if err := page.settings.Toggle(index); err != nil {
		page.status.SetText(err.Error())
		return
	}
	page.persist()
	_ = page.list.SetCurrentIndex(index)
}

func (page *SettingsPage) moveSelected(delta int) {
	index := page.list.CurrentIndex()
	if err := page.settings.Move(index, delta); err != nil {
		page.status.SetText(err.Error())
		return
	}
	if page.persist() {
		_ = page.list.SetCurrentIndex(index + delta)
	}
}

func (page *SettingsPage) addCustom() {
	value, ok := runBootstrapResolverDialog(page.Form())
	if !ok {
		return
	}
	if err := page.settings.AddCustom(value); err != nil {
		page.status.SetText(err.Error())
		return
	}
	if page.persist() {
		_ = page.list.SetCurrentIndex(len(page.settings.Entries) - 1)
	}
}

func (page *SettingsPage) removeSelected() {
	index := page.list.CurrentIndex()
	if err := page.settings.RemoveCustom(index); err != nil {
		page.status.SetText(err.Error())
		return
	}
	if page.persist() {
		if index >= len(page.settings.Entries) {
			index = len(page.settings.Entries) - 1
		}
		_ = page.list.SetCurrentIndex(index)
	}
}

func (page *SettingsPage) restoreDefaults() {
	page.settings.RestoreDefaults()
	page.persist()
}

type bootstrapSettingsModel struct {
	walk.ListModelBase
	entries []bootstrap.Entry
}

func (model *bootstrapSettingsModel) ItemCount() int { return len(model.entries) }

func (model *bootstrapSettingsModel) Value(index int) interface{} {
	entry := model.entries[index]
	state := "Disabled"
	if entry.Enabled {
		state = "Enabled"
	}
	kind := "Built-In"
	if entry.Custom {
		kind = "Custom"
	}
	return fmt.Sprintf("[%s] %s (%s)", state, entry.Address, kind)
}

func runBootstrapResolverDialog(owner walk.Form) (string, bool) {
	dialog, err := walk.NewDialog(owner)
	if err != nil {
		showError(err, owner)
		return "", false
	}
	defer dialog.Dispose()
	dialog.SetTitle(l18n.Sprintf("Add Bootstrap Resolver"))
	applyDarkWindow(dialog.Handle())
	dialog.SetBackground(uiCanvasBrush)
	layout := walk.NewGridLayout()
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	layout.SetSpacing(6)
	dialog.SetLayout(layout)
	label, _ := walk.NewTextLabel(dialog)
	label.SetText(l18n.Sprintf("IP Address:"))
	label.SetTextColor(uiTextColor)
	layout.SetRange(label, walk.Rectangle{0, 0, 1, 1})
	edit, err := walk.NewLineEdit(dialog)
	if err != nil {
		return "", false
	}
	edit.SetCueBanner(l18n.Sprintf("Example: 9.9.9.9 or 2001:4860:4860::8888"))
	edit.SetBackground(uiCardBrush)
	edit.SetTextColor(uiTextColor)
	layout.SetRange(edit, walk.Rectangle{1, 0, 2, 1})
	buttons, _ := walk.NewComposite(dialog)
	buttons.SetLayout(walk.NewHBoxLayout())
	layout.SetRange(buttons, walk.Rectangle{0, 1, 3, 1})
	walk.NewHSpacer(buttons)
	ok, _ := walk.NewPushButton(buttons)
	ok.SetText(l18n.Sprintf("OK"))
	cancel, _ := walk.NewPushButton(buttons)
	cancel.SetText(l18n.Sprintf("Cancel"))
	dialog.SetDefaultButton(ok)
	dialog.SetCancelButton(cancel)
	ok.Clicked().Attach(func() {
		if _, err := bootstrap.ParseAddress(edit.Text()); err != nil {
			showErrorCustom(dialog, l18n.Sprintf("Invalid resolver"), err.Error())
			return
		}
		dialog.Accept()
	})
	if dialog.Run() != walk.DlgCmdOK {
		return "", false
	}
	return edit.Text(), true
}
