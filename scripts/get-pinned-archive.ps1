#requires -Version 7.2
# SPDX-License-Identifier: MIT
# Copyright (C) 2026 WireHush. All Rights Reserved.
# Offline-testable, checksum-verified dependency download shared by build tools.
function Get-PinnedArchive {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string] $Name,
        [Parameter(Mandatory)][string] $Url,
        [Parameter(Mandatory)][string] $Archive,
        [Parameter(Mandatory)][string] $Digest,
        [Parameter(Mandatory)][ValidateSet('SHA256', 'SHA512')][string] $Algorithm,
        [scriptblock] $Download = {
            param([string] $Uri, [string] $OutFile)
            Invoke-WebRequest -Uri $Uri -OutFile $OutFile -TimeoutSec 300 -ErrorAction Stop
        },
        [scriptblock] $Wait = {
            param([int] $Seconds)
            Start-Sleep -Seconds $Seconds
        }
    )

    if (Test-Path -LiteralPath $Archive) {
        if ((Get-FileHash -LiteralPath $Archive -Algorithm $Algorithm).Hash -ne $Digest) {
            throw "Checksum mismatch for cached $Name; delete the invalid archive before retrying."
        }
        return
    }

    # Never expose an incomplete network response as a verified dependency.
    $partial = "$Archive.part"
    $maxAttempts = 4
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        try {
            Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
            & $Download $Url $partial
            if (!(Test-Path -LiteralPath $partial)) {
                throw 'Dependency download returned without a file.'
            }
            if ((Get-FileHash -LiteralPath $partial -Algorithm $Algorithm).Hash -ne $Digest) {
                throw "Checksum mismatch for downloaded $Name; aborting without accepting the archive."
            }
            Move-Item -LiteralPath $partial -Destination $Archive -ErrorAction Stop
            return
        }
        catch {
            $failure = $_
            Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
            if ($failure.Exception.Message -like 'Checksum mismatch*') {
                throw $failure
            }
            # Permanent HTTP client errors (other than request timeout or throttling)
            # must fail immediately rather than repeatedly fetching a missing file.
            $response = $failure.Exception.Response
            if ($null -ne $response -and $null -ne $response.StatusCode) {
                $status = [int] $response.StatusCode
                if ($status -ge 400 -and $status -lt 500 -and $status -notin @(408, 429)) {
                    throw "HTTP $status fetching pinned dependency $Name; no retry for permanent client errors."
                }
            }
            if ($attempt -eq $maxAttempts) {
                throw "Unable to fetch pinned dependency $Name after $maxAttempts attempts. Check the upstream download service or network."
            }
            $delay = [int] [Math]::Pow(2, $attempt)
            Write-Warning "Dependency $Name download failed (attempt $attempt/$maxAttempts); retrying in $delay seconds."
            & $Wait $delay
        }
    }
}
