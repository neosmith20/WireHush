/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
#include <windows.h>
#include <msiquery.h>
#include <sddl.h>
#include <aclapi.h>
#include <lm.h>
#include <shlobj.h>
#include <shellapi.h>
#include <strsafe.h>
#include <stdbool.h>

#define EVENT_NAME L"Global\\WireHush.Installer.Maintenance.v1"
#define SERVICE_NAME L"WireHushManager"
#define WH_GROUP_NAME L"WireHush Users"
#define ARRAY_SIZE(a) (sizeof(a) / sizeof((a)[0]))

static UINT fail(MSIHANDLE session, const WCHAR *category) {
    MSIHANDLE record = MsiCreateRecord(1);
    if (record) {
        MsiRecordSetStringW(record, 0, L"WireHush setup: [1]. No sensitive details were logged.");
        MsiRecordSetStringW(record, 1, category);
        MsiProcessMessage(session, INSTALLMESSAGE_ERROR, record);
        MsiCloseHandle(record);
    }
    return ERROR_INSTALL_FAILURE;
}
static bool property(MSIHANDLE session, const WCHAR *name, WCHAR *value, DWORD count) {
    DWORD size = count;
    return MsiGetPropertyW(session, name, value, &size) == ERROR_SUCCESS;
}
static bool system_owner(HANDLE object) {
    PSID owner = NULL;
    PSECURITY_DESCRIPTOR sd = NULL;
    PSID system = NULL;
    bool ok = GetSecurityInfo(object, SE_KERNEL_OBJECT, OWNER_SECURITY_INFORMATION, &owner, NULL, NULL, NULL, &sd) == ERROR_SUCCESS
        && ConvertStringSidToSidW(L"S-1-5-18", &system) && EqualSid(owner, system);
    if (system) LocalFree(system);
    if (sd) LocalFree(sd);
    return ok;
}
static bool manager_owned(const QUERY_SERVICE_CONFIGW *config) {
    if (config->dwServiceType != SERVICE_WIN32_OWN_PROCESS || _wcsicmp(config->lpServiceStartName, L"LocalSystem")) return false;
    int argc = 0;
    WCHAR **argv = CommandLineToArgvW(config->lpBinaryPathName, &argc);
    bool owned = false;
    PWSTR program_files = NULL;
    if (argv && argc == 2 && !wcscmp(argv[1], L"/managerservice") && SUCCEEDED(SHGetKnownFolderPath(&FOLDERID_ProgramFiles, 0, NULL, &program_files))) {
        const WCHAR *suffixes[] = { L"\\WireHush\\WireHush-Manager.exe", L"\\WireHush\\wirehush.exe", L"\\WireHush\\tunnelmint.exe", L"\\TunnelMint\\wirehush.exe", L"\\TunnelMint\\tunnelmint.exe" };
        for (unsigned i = 0; i < ARRAY_SIZE(suffixes); ++i) {
            WCHAR path[32768];
            if (SUCCEEDED(StringCchPrintfW(path, ARRAY_SIZE(path), L"%s%s", program_files, suffixes[i])) && !_wcsicmp(argv[0], path)) owned = true;
        }
    }
    if (program_files) CoTaskMemFree(program_files);
    if (argv) LocalFree(argv);
    return owned;
}
static QUERY_SERVICE_CONFIGW *read_config(SC_HANDLE service) {
    DWORD size = 0;
    QueryServiceConfigW(service, NULL, 0, &size);
    if (!size || size > 65536) return NULL;
    QUERY_SERVICE_CONFIGW *config = HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, size);
    if (config && !QueryServiceConfigW(service, config, size, &size)) { HeapFree(GetProcessHeap(), 0, config); config = NULL; }
    return config;
}

__declspec(dllexport) UINT __stdcall CaptureMaintenance(MSIHANDLE session) {
    SYSTEM_INFO system;
    GetNativeSystemInfo(&system);
#if defined(__aarch64__)
    if (system.wProcessorArchitecture != PROCESSOR_ARCHITECTURE_ARM64) return fail(session, L"use the installer matching the native processor architecture");
#else
    if (system.wProcessorArchitecture != PROCESSOR_ARCHITECTURE_AMD64) return fail(session, L"use the installer matching the native processor architecture");
#endif
    PWSTR program_files = NULL;
    WCHAR expected[32768], actual[32768];
    bool canonical = SUCCEEDED(SHGetKnownFolderPath(&FOLDERID_ProgramFiles, 0, NULL, &program_files))
        && SUCCEEDED(StringCchPrintfW(expected, ARRAY_SIZE(expected), L"%s\\WireHush\\", program_files))
        && property(session, L"INSTALLFOLDER", actual, ARRAY_SIZE(actual)) && !_wcsicmp(expected, actual);
    if (program_files) CoTaskMemFree(program_files);
    if (!canonical) return fail(session, L"installation requires the standard protected Program Files location");
    SC_HANDLE scm = OpenSCManagerW(NULL, NULL, SC_MANAGER_CONNECT);
    if (!scm) return fail(session, L"service ownership could not be checked");
    SC_HANDLE service = OpenServiceW(scm, SERVICE_NAME, SERVICE_QUERY_CONFIG);
    WCHAR saved[32768] = L"";
    bool ok = true;
    if (service) {
        QUERY_SERVICE_CONFIGW *config = read_config(service);
        ok = config && manager_owned(config) && SUCCEEDED(StringCchPrintfW(saved, ARRAY_SIZE(saved), L"%lu|%s", config->dwStartType, config->lpBinaryPathName));
        if (config) HeapFree(GetProcessHeap(), 0, config);
        CloseServiceHandle(service);
    } else if (GetLastError() != ERROR_SERVICE_DOES_NOT_EXIST) ok = false;
    CloseServiceHandle(scm);
    if (!ok) return fail(session, L"a service ownership conflict requires administrator review");
    if (MsiSetPropertyW(session, L"RollbackMaintenance", saved) || MsiSetPropertyW(session, L"BeginMaintenance", saved)) return fail(session, L"maintenance state could not be saved");
    return ERROR_SUCCESS;
}

__declspec(dllexport) UINT __stdcall BeginMaintenance(MSIHANDLE session) {
    PSECURITY_DESCRIPTOR sd = NULL;
    if (!ConvertStringSecurityDescriptorToSecurityDescriptorW(L"O:SYG:SYD:P(A;;GA;;;SY)(A;;GA;;;BA)", SDDL_REVISION_1, &sd, NULL)) return fail(session, L"maintenance protection failed");
    SECURITY_ATTRIBUTES attributes = { sizeof(attributes), sd, FALSE };
    HANDLE event = CreateEventW(&attributes, TRUE, FALSE, EVENT_NAME);
    LocalFree(sd);
    if (!event || !system_owner(event) || !SetEvent(event)) { if (event) CloseHandle(event); return fail(session, L"maintenance protection failed"); }
    // Intentionally retain this handle in the MSI host until transaction end.
    // Commit/rollback clears the event; host termination releases the last handle.
    SC_HANDLE scm = OpenSCManagerW(NULL, NULL, SC_MANAGER_CONNECT);
    if (!scm) return fail(session, L"service maintenance failed");
    SC_HANDLE service = OpenServiceW(scm, SERVICE_NAME, SERVICE_QUERY_CONFIG | SERVICE_CHANGE_CONFIG | SERVICE_STOP | SERVICE_QUERY_STATUS);
    bool ok = true;
    if (service) {
        QUERY_SERVICE_CONFIGW *config = read_config(service);
        ok = config && manager_owned(config) && ChangeServiceConfigW(service, SERVICE_NO_CHANGE, SERVICE_DISABLED, SERVICE_NO_CHANGE, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
        if (config) HeapFree(GetProcessHeap(), 0, config);
        SERVICE_STATUS status;
        if (ok && !ControlService(service, SERVICE_CONTROL_STOP, &status) && GetLastError() != ERROR_SERVICE_NOT_ACTIVE) ok = false;
        ULONGLONG end = GetTickCount64() + 15000;
        while (ok) {
            if (!QueryServiceStatus(service, &status)) { ok = false; break; }
            if (status.dwCurrentState == SERVICE_STOPPED) { ok = status.dwWin32ExitCode == 0 || status.dwWin32ExitCode == ERROR_SERVICE_NEVER_STARTED; break; }
            if (GetTickCount64() >= end) { ok = false; break; }
            Sleep(50);
        }
        CloseServiceHandle(service);
    } else if (GetLastError() != ERROR_SERVICE_DOES_NOT_EXIST) ok = false;
    CloseServiceHandle(scm);
    return ok ? ERROR_SUCCESS : fail(session, L"network cleanup must complete before installation can continue");
}

static UINT finish_maintenance(MSIHANDLE session, bool rollback) {
    SC_HANDLE scm = OpenSCManagerW(NULL, NULL, SC_MANAGER_CONNECT | SC_MANAGER_CREATE_SERVICE);
    if (!scm) return fail(session, L"service maintenance could not be completed");
    SC_HANDLE service = OpenServiceW(scm, SERVICE_NAME, SERVICE_QUERY_CONFIG | SERVICE_CHANGE_CONFIG);
    WCHAR saved[32768] = L"";
    bool ok = true;
    if (rollback && !property(session, L"CustomActionData", saved, ARRAY_SIZE(saved))) ok = false;
    WCHAR *binary = wcschr(saved, L'|');
    DWORD start = SERVICE_DEMAND_START;
    if (rollback && binary) { *binary++ = 0; start = wcstoul(saved, NULL, 10); }
    if (service) {
        QUERY_SERVICE_CONFIGW *config = read_config(service);
        ok = ok && config && manager_owned(config) && ChangeServiceConfigW(service, SERVICE_NO_CHANGE, start, SERVICE_NO_CHANGE, rollback && binary ? binary : NULL, NULL, NULL, NULL, NULL, NULL, NULL);
        if (config) HeapFree(GetProcessHeap(), 0, config);
        CloseServiceHandle(service);
    } else if (rollback && binary && GetLastError() == ERROR_SERVICE_DOES_NOT_EXIST) {
        service = CreateServiceW(scm, SERVICE_NAME, L"WireHush Manager", SERVICE_QUERY_CONFIG, SERVICE_WIN32_OWN_PROCESS, start, SERVICE_ERROR_NORMAL, binary, NULL, NULL, NULL, L"LocalSystem", NULL);
        if (!service) ok = false; else CloseServiceHandle(service);
    } else if (GetLastError() != ERROR_SERVICE_DOES_NOT_EXIST) ok = false;
    CloseServiceHandle(scm);
    HANDLE event = OpenEventW(EVENT_MODIFY_STATE | READ_CONTROL, FALSE, EVENT_NAME);
    if (event) { if (!system_owner(event) || !ResetEvent(event)) ok = false; CloseHandle(event); }
    else if (GetLastError() != ERROR_FILE_NOT_FOUND) ok = false;
    return ok ? ERROR_SUCCESS : fail(session, L"service maintenance could not be completed");
}
__declspec(dllexport) UINT __stdcall RollbackMaintenance(MSIHANDLE session) { return finish_maintenance(session, true); }
__declspec(dllexport) UINT __stdcall CommitMaintenance(MSIHANDLE session) { return finish_maintenance(session, false); }

__declspec(dllexport) UINT __stdcall ProvisionAccess(MSIHANDLE session) {
    LOCALGROUP_INFO_1 group = { WH_GROUP_NAME, L"Users permitted to control WireHush tunnels" };
    NET_API_STATUS status = NetLocalGroupAdd(NULL, 1, (BYTE *)&group, NULL);
    if (status != NERR_Success && status != NERR_GroupExists && status != ERROR_ALIAS_EXISTS) return fail(session, L"local user group could not be provisioned");
    WCHAR computer[MAX_COMPUTERNAME_LENGTH + 1], account[256], domain[256];
    DWORD computer_size = ARRAY_SIZE(computer), sid_size = SECURITY_MAX_SID_SIZE, domain_size = ARRAY_SIZE(domain);
    BYTE group_sid[SECURITY_MAX_SID_SIZE];
    SID_NAME_USE kind;
    if (!GetComputerNameW(computer, &computer_size) || FAILED(StringCchPrintfW(account, ARRAY_SIZE(account), L"%s\\%s", computer, WH_GROUP_NAME)) || !LookupAccountNameW(NULL, account, group_sid, &sid_size, domain, &domain_size, &kind) || kind != SidTypeAlias) return fail(session, L"local group identity could not be verified");
    WCHAR caller[256];
    PSID caller_sid = NULL;
    if (!property(session, L"CustomActionData", caller, ARRAY_SIZE(caller)) || !ConvertStringSidToSidW(caller, &caller_sid)) return fail(session, L"installer user identity is unavailable");
    if (wcscmp(caller, L"S-1-5-18")) {
        LOCALGROUP_MEMBERS_INFO_0 member = { caller_sid };
        status = NetLocalGroupAddMembers(NULL, WH_GROUP_NAME, 0, (BYTE *)&member, 1);
        if (status != NERR_Success && status != ERROR_MEMBER_IN_ALIAS) { LocalFree(caller_sid); return fail(session, L"installer user could not be granted access"); }
    }
    LocalFree(caller_sid);
    WCHAR *sid_text = NULL, sddl[512];
    PSECURITY_DESCRIPTOR sd = NULL;
    if (!ConvertSidToStringSidW(group_sid, &sid_text)) return fail(session, L"local group security could not be prepared");
    bool ok = SUCCEEDED(StringCchPrintfW(sddl, ARRAY_SIZE(sddl), L"O:SYG:SYD:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;LCRPRC;;;%s)", sid_text)) && ConvertStringSecurityDescriptorToSecurityDescriptorW(sddl, SDDL_REVISION_1, &sd, NULL);
    LocalFree(sid_text);
    SC_HANDLE scm = OpenSCManagerW(NULL, NULL, SC_MANAGER_CONNECT | SC_MANAGER_CREATE_SERVICE);
    SC_HANDLE service = scm ? OpenServiceW(scm, SERVICE_NAME, SERVICE_QUERY_CONFIG | SERVICE_CHANGE_CONFIG | WRITE_DAC | WRITE_OWNER) : NULL;
    PWSTR program_files = NULL;
    WCHAR binary[32768];
    ok = ok && SUCCEEDED(SHGetKnownFolderPath(&FOLDERID_ProgramFiles, 0, NULL, &program_files))
        && SUCCEEDED(StringCchPrintfW(binary, ARRAY_SIZE(binary), L"\"%s\\WireHush\\WireHush-Manager.exe\" /managerservice", program_files));
    if (program_files) CoTaskMemFree(program_files);
    // A predecessor MSI can remove its dynamic registration after the new
    // InstallServices action. Commit re-establishes only this canonical service.
    if (ok && scm && !service) {
            service = CreateServiceW(scm, SERVICE_NAME, L"WireHush Manager", SERVICE_QUERY_CONFIG | SERVICE_CHANGE_CONFIG | WRITE_DAC | WRITE_OWNER, SERVICE_WIN32_OWN_PROCESS, SERVICE_DEMAND_START, SERVICE_ERROR_NORMAL, binary, NULL, NULL, NULL, L"LocalSystem", NULL);
    }
    QUERY_SERVICE_CONFIGW *config = service ? read_config(service) : NULL;
    SERVICE_SID_INFO service_sid = { SERVICE_SID_TYPE_UNRESTRICTED };
    WCHAR privileges[] = L"SeChangeNotifyPrivilege\0SeImpersonatePrivilege\0";
    SERVICE_REQUIRED_PRIVILEGES_INFOW required = { privileges };
    ok = ok && config && manager_owned(config)
        && ChangeServiceConfigW(service, SERVICE_WIN32_OWN_PROCESS, SERVICE_DEMAND_START, SERVICE_ERROR_NORMAL, binary, NULL, NULL, NULL, L"LocalSystem", NULL, L"WireHush Manager")
        && SetServiceObjectSecurity(service, OWNER_SECURITY_INFORMATION | GROUP_SECURITY_INFORMATION | DACL_SECURITY_INFORMATION, sd)
        && ChangeServiceConfig2W(service, SERVICE_CONFIG_SERVICE_SID_INFO, &service_sid)
        && ChangeServiceConfig2W(service, SERVICE_CONFIG_REQUIRED_PRIVILEGES_INFO, &required);
    if (config) HeapFree(GetProcessHeap(), 0, config);
    if (service) CloseServiceHandle(service);
    if (scm) CloseServiceHandle(scm);
    if (sd) LocalFree(sd);
    return ok ? ERROR_SUCCESS : fail(session, L"manager service security could not be provisioned");
}

// Only fixed known-folder/profile roots reach this routine. Every opened object
// stays pinned against rename/delete, is checked by handle, and is never followed
// through a reparse point. A complete validation pass precedes the deletion pass.
static ULONGLONG deletion_deadline;
static bool owned_tree(const WCHAR *path, PSID expected_owner, bool remove, unsigned depth, unsigned *count) {
    if (depth > 64 || ++*count > 100000) return false;
    if (deletion_deadline && GetTickCount64() >= deletion_deadline) return false;
    HANDLE file = CreateFileW(path, FILE_READ_ATTRIBUTES | READ_CONTROL | (remove ? DELETE : 0), FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_EXISTING, FILE_FLAG_OPEN_REPARSE_POINT | FILE_FLAG_BACKUP_SEMANTICS, NULL);
    if (file == INVALID_HANDLE_VALUE) return GetLastError() == ERROR_FILE_NOT_FOUND || GetLastError() == ERROR_PATH_NOT_FOUND;
    BY_HANDLE_FILE_INFORMATION info;
    WCHAR *final = HeapAlloc(GetProcessHeap(), 0, 32768 * sizeof(WCHAR));
    WCHAR *normalized = HeapAlloc(GetProcessHeap(), 0, 32768 * sizeof(WCHAR));
    PSECURITY_DESCRIPTOR sd = NULL;
    PSID owner = NULL;
    bool ok = final && normalized && GetFileInformationByHandle(file, &info)
        && !(info.dwFileAttributes & FILE_ATTRIBUTE_REPARSE_POINT)
        && ((info.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) || info.nNumberOfLinks == 1)
        && GetSecurityInfo(file, SE_FILE_OBJECT, OWNER_SECURITY_INFORMATION, &owner, NULL, NULL, NULL, &sd) == ERROR_SUCCESS
        && EqualSid(owner, expected_owner)
        && SUCCEEDED(StringCchPrintfW(normalized, 32768, L"\\\\?\\%s", path));
    DWORD final_length = ok ? GetFinalPathNameByHandleW(file, final, 32768, FILE_NAME_NORMALIZED | VOLUME_NAME_DOS) : 0;
    ok = ok && final_length > 0 && final_length < 32768 && !_wcsicmp(final, normalized);
    if (sd) LocalFree(sd);
    if (ok && (info.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) {
        WCHAR *child = HeapAlloc(GetProcessHeap(), 0, 32768 * sizeof(WCHAR));
        WIN32_FIND_DATAW entry;
        HANDLE search = INVALID_HANDLE_VALUE;
        ok = child && SUCCEEDED(StringCchPrintfW(child, 32768, L"%s\\*", path));
        if (ok) search = FindFirstFileW(child, &entry);
        if (search == INVALID_HANDLE_VALUE && GetLastError() != ERROR_FILE_NOT_FOUND) ok = false;
        if (search != INVALID_HANDLE_VALUE) {
            do {
                if (!wcscmp(entry.cFileName, L".") || !wcscmp(entry.cFileName, L"..")) continue;
                ok = SUCCEEDED(StringCchPrintfW(child, 32768, L"%s\\%s", path, entry.cFileName)) && owned_tree(child, expected_owner, remove, depth + 1, count);
            } while (ok && FindNextFileW(search, &entry));
            if (ok && GetLastError() != ERROR_NO_MORE_FILES) ok = false;
            FindClose(search);
        }
        if (child) HeapFree(GetProcessHeap(), 0, child);
    }
    if (ok && remove) { FILE_DISPOSITION_INFO disposition = { TRUE }; ok = SetFileInformationByHandle(file, FileDispositionInfo, &disposition, sizeof(disposition)); }
    CloseHandle(file);
    if (final) HeapFree(GetProcessHeap(), 0, final);
    if (normalized) HeapFree(GetProcessHeap(), 0, normalized);
    return ok;
}
static bool delete_root(const WCHAR *path, const WCHAR *owner_text) {
    PSID owner = NULL;
    if (!ConvertStringSidToSidW(owner_text, &owner)) return false;
    unsigned count = 0;
    bool ok = owned_tree(path, owner, false, 0, &count);
    count = 0;
    if (ok) ok = owned_tree(path, owner, true, 0, &count);
    LocalFree(owner);
    return ok;
}
__declspec(dllexport) UINT __stdcall DeleteOwnedData(MSIHANDLE session) {
    deletion_deadline = GetTickCount64() + 120000;
    PWSTR data = NULL;
    WCHAR *path = HeapAlloc(GetProcessHeap(), 0, 32768 * sizeof(WCHAR));
    if (!path || FAILED(SHGetKnownFolderPath(&FOLDERID_ProgramData, 0, NULL, &data))) { if (path) HeapFree(GetProcessHeap(), 0, path); return fail(session, L"data cleanup could not resolve the protected root"); }
    bool ok = SUCCEEDED(StringCchPrintfW(path, 32768, L"%s\\WireHush", data)) && delete_root(path, L"S-1-5-18");
    CoTaskMemFree(data);
    HKEY profiles = NULL;
    if (ok && RegOpenKeyExW(HKEY_LOCAL_MACHINE, L"SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\ProfileList", 0, KEY_READ | KEY_WOW64_64KEY, &profiles) != ERROR_SUCCESS) ok = false;
    for (DWORD index = 0; ok && profiles; ++index) {
        WCHAR sid[256], profile[32768], expanded[32768];
        DWORD sid_length = ARRAY_SIZE(sid);
        LSTATUS status = RegEnumKeyExW(profiles, index, sid, &sid_length, NULL, NULL, NULL, NULL);
        if (status == ERROR_NO_MORE_ITEMS) break;
        if (status != ERROR_SUCCESS) { ok = false; break; }
        if (wcsncmp(sid, L"S-1-5-21-", 9) && wcsncmp(sid, L"S-1-12-1-", 9)) continue;
        HKEY profile_key;
        if (RegOpenKeyExW(profiles, sid, 0, KEY_READ, &profile_key) != ERROR_SUCCESS) { ok = false; break; }
        DWORD size = sizeof(profile), type;
        status = RegQueryValueExW(profile_key, L"ProfileImagePath", NULL, &type, (BYTE *)profile, &size);
        RegCloseKey(profile_key);
        if (status != ERROR_SUCCESS || (type != REG_SZ && type != REG_EXPAND_SZ) || size < sizeof(WCHAR) || size > sizeof(profile) || profile[size / sizeof(WCHAR) - 1]) { ok = false; break; }
        DWORD length = ExpandEnvironmentStringsW(profile, expanded, ARRAY_SIZE(expanded));
        if (!length || length > ARRAY_SIZE(expanded) || expanded[1] != L':' || expanded[2] != L'\\' || FAILED(StringCchPrintfW(path, 32768, L"%s\\AppData\\Local\\WireHush", expanded)) || !delete_root(path, sid)) { ok = false; break; }
    }
    if (profiles) RegCloseKey(profiles);
    HeapFree(GetProcessHeap(), 0, path);
    return ok ? ERROR_SUCCESS : fail(session, L"data cleanup refused an unsafe or inaccessible object; retained data requires owner review");
}
