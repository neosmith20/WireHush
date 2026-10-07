/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
#include "v1-customactions.c"
#include <stdio.h>

#define REQUIRE(condition) do { if (!(condition)) { fwprintf(stderr, L"Installer safety check failed at line %d\n", __LINE__); return 1; } } while (0)
int wmain(void) {
    WCHAR temporary[MAX_PATH], root[MAX_PATH], child[MAX_PATH], outside[MAX_PATH];
    REQUIRE(GetTempPathW(ARRAY_SIZE(temporary), temporary));
    REQUIRE(SUCCEEDED(StringCchPrintfW(root, ARRAY_SIZE(root), L"%sWireHush.Installer.Test.%lu.%llu", temporary, GetCurrentProcessId(), GetTickCount64())));
    REQUIRE(CreateDirectoryW(root, NULL));
    HANDLE root_handle = CreateFileW(root, READ_CONTROL, FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_EXISTING, FILE_FLAG_BACKUP_SEMANTICS, NULL);
    REQUIRE(root_handle != INVALID_HANDLE_VALUE);
    WCHAR final[MAX_PATH];
    REQUIRE(GetFinalPathNameByHandleW(root_handle, final, ARRAY_SIZE(final), FILE_NAME_NORMALIZED | VOLUME_NAME_DOS));
    REQUIRE(SUCCEEDED(StringCchCopyW(root, ARRAY_SIZE(root), final + 4)));
    PSID root_owner;
    PSECURITY_DESCRIPTOR root_sd = NULL;
    REQUIRE(GetSecurityInfo(root_handle, SE_FILE_OBJECT, OWNER_SECURITY_INFORMATION, &root_owner, NULL, NULL, NULL, &root_sd) == ERROR_SUCCESS);
    CloseHandle(root_handle);
    REQUIRE(SUCCEEDED(StringCchPrintfW(child, ARRAY_SIZE(child), L"%s\\test.txt", root)));
    HANDLE file = CreateFileW(child, GENERIC_WRITE, 0, NULL, CREATE_NEW, FILE_ATTRIBUTE_NORMAL, NULL);
    REQUIRE(file != INVALID_HANDLE_VALUE);
    CloseHandle(file);
    unsigned count = 0;
    REQUIRE(owned_tree(root, root_owner, false, 0, &count));
    REQUIRE(SUCCEEDED(StringCchPrintfW(outside, ARRAY_SIZE(outside), L"%s.outside.txt", root)));
    REQUIRE(CreateHardLinkW(outside, child, NULL));
    count = 0;
    REQUIRE(!owned_tree(root, root_owner, false, 0, &count));
    REQUIRE(GetFileAttributesW(outside) != INVALID_FILE_ATTRIBUTES);
    REQUIRE(DeleteFileW(outside));
    PSID foreign;
    REQUIRE(ConvertStringSidToSidW(L"S-1-5-7", &foreign));
    count = 0;
    REQUIRE(!owned_tree(root, foreign, true, 0, &count));
    REQUIRE(GetFileAttributesW(child) != INVALID_FILE_ATTRIBUTES);
    LocalFree(foreign);
    count = 0;
    REQUIRE(owned_tree(root, root_owner, true, 0, &count));
    LocalFree(root_sd);
    REQUIRE(GetFileAttributesW(root) == INVALID_FILE_ATTRIBUTES);
    QUERY_SERVICE_CONFIGW config = {0};
    config.dwServiceType = SERVICE_WIN32_OWN_PROCESS;
    config.lpServiceStartName = L"LocalSystem";
    config.lpBinaryPathName = L"\"C:\\Unrelated\\WireHush-Manager.exe\" /managerservice";
    REQUIRE(!manager_owned(&config));
    PWSTR program_files = NULL;
    WCHAR binary[32768];
    REQUIRE(SUCCEEDED(SHGetKnownFolderPath(&FOLDERID_ProgramFiles, 0, NULL, &program_files)));
    REQUIRE(SUCCEEDED(StringCchPrintfW(binary, ARRAY_SIZE(binary), L"\"%s\\WireHush\\WireHush-Manager.exe\" /managerservice", program_files)));
    CoTaskMemFree(program_files);
    config.lpBinaryPathName = binary;
    REQUIRE(manager_owned(&config));
    config.lpServiceStartName = L"OtherAccount";
    REQUIRE(!manager_owned(&config));
    wprintf(L"PASS: canonical service ownership, hard-link rejection, foreign-owner rejection, safe handle-based deletion\n");
    return 0;
}
