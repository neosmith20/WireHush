/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package conf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/bootstrap"
	"golang.zx2c4.com/wireguard/windows/conf/dpapi"
	"os"
	"path/filepath"
	"strings"
)

const wireHushMigrationDescription = "WireHush V1 Migration Journal"

type LegacyMigrationSource struct {
	Product, FileName, Name, Digest, WGQuickText string
	EncryptedBackup                              []byte
}

func WireHushLegacyRoots() (map[string]string, error) {
	base, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return nil, err
	}
	return map[string]string{"TunnelMint": filepath.Join(base, "TunnelMint", "Data"), "WireHush": filepath.Join(base, "WireHush", "Data")}, nil
}
func verifyLegacyMigrationDirectory(path string) error {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(ptr, windows.READ_CONTROL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return errors.New("legacy migration directory is not trusted")
	}
	return verifyFinalPath(handle, path)
}
func ReadWireHushLegacyMigrationSources() ([]LegacyMigrationSource, error) {
	roots, err := WireHushLegacyRoots()
	if err != nil {
		return nil, err
	}
	var sources []LegacyMigrationSource
	for _, product := range []string{"TunnelMint", "WireHush"} {
		root := roots[product]
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		for _, path := range []string{filepath.Dir(root), root} {
			if err := verifyLegacyMigrationDirectory(path); err != nil {
				return nil, err
			}
		}
		directory := filepath.Join(root, "Configurations")
		if _, err := os.Stat(directory); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		if err := verifyLegacyMigrationDirectory(directory); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, err
		}
		if len(entries) > 4096 {
			return nil, errors.New("legacy source count exceeds migration limit")
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name, err := NameFromPath(entry.Name())
			if err != nil {
				continue
			}
			raw, err := readVerifiedTunnelRecordFile(filepath.Join(directory, entry.Name()))
			if err != nil {
				return nil, err
			}
			digest := sha256.Sum256(raw)
			plaintext, backup := raw, raw
			if strings.HasSuffix(entry.Name(), configFileSuffix) {
				plaintext, err = dpapi.Decrypt(raw, name)
			} else {
				backup, err = dpapi.Encrypt(raw, name)
			}
			if err != nil {
				return nil, err
			}
			config, err := FromWgQuickWithUnknownEncoding(string(plaintext), name)
			if err != nil {
				return nil, err
			}
			text := config.ToWgQuick()
			if len(text) > 1<<20 || len(sources) >= 4096 {
				return nil, errors.New("legacy migration exceeds V1 storage limits")
			}
			sources = append(sources, LegacyMigrationSource{Product: product, FileName: entry.Name(), Name: name, Digest: hex.EncodeToString(digest[:]), WGQuickText: text, EncryptedBackup: backup})
		}
	}
	return sources, nil
}
func BackupWireHushLegacySource(source LegacyMigrationSource) error {
	if source.Product != "TunnelMint" && source.Product != "WireHush" {
		return errors.New("invalid legacy source")
	}
	if !TunnelNameIsValid(source.Name) || len(source.EncryptedBackup) == 0 {
		return errors.New("invalid legacy backup")
	}
	digest, err := hex.DecodeString(source.Digest)
	if err != nil || len(digest) != sha256.Size || strings.ToLower(source.Digest) != source.Digest {
		return errors.New("invalid legacy backup digest")
	}
	root, err := PrepareWireHushMachineData()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString(tunnelRecordDirectorySDDL)
	if err != nil {
		return err
	}
	for _, path := range []string{filepath.Join(root, "Migration", "Backups"), filepath.Join(root, "Migration", "Backups", source.Product)} {
		if err := ensureVerifiedTunnelRecordDirectory(path, sd); err != nil {
			return err
		}
	}
	destination := filepath.Join(root, "Migration", "Backups", source.Product, source.Name+"-"+source.Digest+configFileSuffix)
	if existing, err := readVerifiedTunnelRecordFile(destination); err == nil {
		original, err := dpapi.Decrypt(existing, source.Name)
		if err != nil {
			return err
		}
		config, err := FromWgQuickWithUnknownEncoding(string(original), source.Name)
		if err != nil || config.ToWgQuick() != source.WGQuickText {
			return errors.New("legacy backup validation failed")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return err
	}
	if err := writeLockedDownFile(destination, false, source.EncryptedBackup); err != nil {
		return err
	}
	verified, err := readVerifiedTunnelRecordFile(destination)
	if err != nil {
		return err
	}
	plaintext, err := dpapi.Decrypt(verified, source.Name)
	if err != nil {
		return err
	}
	config, err := FromWgQuickWithUnknownEncoding(string(plaintext), source.Name)
	if err != nil {
		return err
	}
	if config.ToWgQuick() != source.WGQuickText {
		return errors.New("legacy backup read-back did not match its source")
	}
	return nil
}

func MigrateWireHushLegacyBootstrapSettings() error {
	root, err := PrepareWireHushMachineData()
	if err != nil {
		return err
	}
	_, currentErr := readVerifiedTunnelRecordFile(filepath.Join(root, "Settings", "bootstrap-dns.dpapi"))
	currentExists := currentErr == nil
	if currentErr != nil && !errors.Is(currentErr, os.ErrNotExist) && !errors.Is(currentErr, windows.ERROR_FILE_NOT_FOUND) {
		return currentErr
	}
	if currentExists {
		if _, err := loadWireHushBootstrapAtRoot(root); err != nil {
			return err
		}
	}
	roots, err := WireHushLegacyRoots()
	if err != nil {
		return err
	}
	var selected []byte
	for _, product := range []string{"TunnelMint", "WireHush"} {
		legacyRoot := roots[product]
		if _, err := os.Stat(legacyRoot); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		for _, directory := range []string{filepath.Dir(legacyRoot), legacyRoot} {
			if err := verifyLegacyMigrationDirectory(directory); err != nil {
				return err
			}
		}
		raw, err := readVerifiedTunnelRecordFile(filepath.Join(legacyRoot, "bootstrap-dns.json"))
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			continue
		}
		if err != nil {
			return err
		}
		settings, err := bootstrap.ParseSettings(raw)
		if err != nil {
			return err
		}
		normalized, err := bootstrap.MarshalSettings(settings)
		if err != nil {
			return err
		}
		sd, err := windows.SecurityDescriptorFromString(tunnelRecordDirectorySDDL)
		if err != nil {
			return err
		}
		directory := filepath.Join(root, "Migration", "Backups")
		if err := ensureVerifiedTunnelRecordDirectory(directory, sd); err != nil {
			return err
		}
		directory = filepath.Join(directory, product)
		if err := ensureVerifiedTunnelRecordDirectory(directory, sd); err != nil {
			return err
		}
		digest := sha256.Sum256(raw)
		backup := filepath.Join(directory, "bootstrap-"+hex.EncodeToString(digest[:])+".dpapi")
		if _, err := readVerifiedTunnelRecordFile(backup); errors.Is(err, os.ErrNotExist) || errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			ciphertext, err := dpapi.Encrypt(raw, "WireHush Legacy Bootstrap "+product)
			if err != nil {
				return err
			}
			if err := writeLockedDownFile(backup, false, ciphertext); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if currentExists {
			continue
		}
		if selected != nil && !bytes.Equal(selected, normalized) {
			return errors.New("conflicting legacy bootstrap settings require owner review")
		}
		selected = normalized
	}
	if !currentExists && selected != nil {
		settings, err := bootstrap.ParseSettings(selected)
		if err != nil {
			return err
		}
		return saveWireHushBootstrapAtRoot(root, settings)
	}
	return nil
}
func LoadWireHushMigrationJournal() ([]byte, error) {
	root, err := PrepareWireHushMachineData()
	if err != nil {
		return nil, err
	}
	data, err := readVerifiedTunnelRecordFile(filepath.Join(root, "Migration", "journal.dpapi"))
	if err != nil {
		return nil, err
	}
	return dpapi.Decrypt(data, wireHushMigrationDescription)
}
func SaveWireHushMigrationJournal(data []byte) error {
	if len(data) > 4*1024*1024 {
		return errors.New("migration journal exceeds size limit")
	}
	root, err := PrepareWireHushMachineData()
	if err != nil {
		return err
	}
	ciphertext, err := dpapi.Encrypt(data, wireHushMigrationDescription)
	if err != nil {
		return err
	}
	return writeLockedDownFile(filepath.Join(root, "Migration", "journal.dpapi"), true, ciphertext)
}
