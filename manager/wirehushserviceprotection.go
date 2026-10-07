/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

const wireHushWorkerServiceSDDL = "O:SYG:SYD:P(A;;GA;;;SY)(A;;GA;;;BA)"

func protectWireHushWorkerService(service *mgr.Service) error {
	sd, err := windows.SecurityDescriptorFromString(wireHushWorkerServiceSDDL)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	group, _, err := sd.Group()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(service.Handle, windows.SE_SERVICE, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, owner, group, dacl, nil)
}

// CreateService cannot receive a security descriptor. Its initial binding is a
// harmless unsupported command, with no owner identity. Publish the canonical
// locator only after applying the protected DACL, and start only after binding.
func finishWireHushWorkerProvision(protect, bind, start func() error) error {
	if err := protect(); err != nil {
		return err
	}
	if err := bind(); err != nil {
		return err
	}
	return start()
}
