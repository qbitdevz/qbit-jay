# qbit-jay installer for Windows (PowerShell 5.1 and 7). No administrator rights.
#
#   irm https://raw.githubusercontent.com/qbitdevz/qbit-jay/main/scripts/install.ps1 | iex
#
# Downloads checksums.txt and the windows zip for this architecture from a
# GitHub Release of qbitdevz/qbit-jay, verifies the zip's sha256 with
# Get-FileHash, and only then installs qbit-jay.exe into
# %LOCALAPPDATA%\Programs\qbit-jay and adds that folder to the user PATH.
# Nothing is installed when the checksum does not match.
#
# Environment (a piped script cannot take parameters):
#   QBIT_JAY_VERSION           release tag to install (default: latest)
#   QBIT_JAY_INSTALL_DIR       target folder (default: %LOCALAPPDATA%\Programs\qbit-jay)
#   QBIT_JAY_INSTALL_BASE_URL  folder URL holding checksums.txt and the zip
#                              (https://, http:// or file://); for mirrors and tests
#   QBIT_JAY_NO_MODIFY_PATH    set to 1 to leave the user PATH alone
#
# Limits: checksums.txt and the zip come from the same origin, so this detects
# corrupt or truncated downloads and mismatched assets, not a compromised release.

function Install-QbitJay {
    $ErrorActionPreference = 'Stop'
    $repo = 'qbitdevz/qbit-jay'
    $bin = 'qbit-jay.exe'

    $version = if ($env:QBIT_JAY_VERSION) { $env:QBIT_JAY_VERSION } else { 'latest' }
    $installDir = $env:QBIT_JAY_INSTALL_DIR
    if (-not $installDir) {
        if (-not $env:LOCALAPPDATA) { throw 'LOCALAPPDATA is not set; set QBIT_JAY_INSTALL_DIR' }
        $installDir = Join-Path $env:LOCALAPPDATA 'Programs\qbit-jay'
    }

    $archName = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    switch ($archName) {
        'AMD64' { $arch = 'amd64' }
        'ARM64' { $arch = 'arm64' }
        default { throw "unsupported architecture: $archName" }
    }

    if ($env:QBIT_JAY_INSTALL_BASE_URL) {
        $base = $env:QBIT_JAY_INSTALL_BASE_URL.TrimEnd('/')
    } elseif ($version -eq 'latest') {
        $base = "https://github.com/$repo/releases/latest/download"
    } else {
        $tag = if ($version.StartsWith('v')) { $version } else { "v$version" }
        if ($tag -notmatch '^[A-Za-z0-9._+-]+$') { throw "invalid version: $version" }
        $base = "https://github.com/$repo/releases/download/$tag"
    }

    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }

    function Get-Resource([string]$Url, [string]$Dest) {
        if ($Url -match '^file://') {
            Copy-Item -LiteralPath ([Uri]$Url).LocalPath -Destination $Dest -ErrorAction Stop
        } else {
            $previous = $ProgressPreference
            $ProgressPreference = 'SilentlyContinue'  # progress rendering makes 5.1 downloads very slow
            try { Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $Dest -ErrorAction Stop }
            finally { $ProgressPreference = $previous }
        }
    }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('qbit-jay-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading checksums.txt ($base)"
        $sums = Join-Path $tmp 'checksums.txt'
        try { Get-Resource "$base/checksums.txt" $sums } catch { throw "cannot download $base/checksums.txt ($($_.Exception.Message))" }

        # The archive name carries the version, so take it from checksums.txt.
        $pattern = '^([0-9a-fA-F]{64})\s+(qbit-jay_[0-9A-Za-z.+-]+_windows_' + $arch + '\.zip)\s*$'
        $found = @(Get-Content -LiteralPath $sums | ForEach-Object { if ($_ -match $pattern) { , @($Matches[1], $Matches[2]) } })
        if ($found.Count -eq 0) { throw "no windows/$arch archive listed in checksums.txt" }
        if ($found.Count -gt 1) { throw "more than one windows/$arch archive listed in checksums.txt" }
        $want = $found[0][0].ToLowerInvariant()
        $archive = $found[0][1]

        Write-Host "Downloading $archive"
        $zip = Join-Path $tmp $archive
        try { Get-Resource "$base/$archive" $zip } catch { throw "cannot download $base/$archive ($($_.Exception.Message))" }

        $got = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($got -ne $want) { throw "checksum mismatch for $archive (expected $want, got $got); nothing was installed" }
        Write-Host "Checksum verified (sha256 $got)"

        # Read the one entry we need instead of expanding the whole archive.
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $staged = Join-Path $tmp $bin
        $reader = [IO.Compression.ZipFile]::OpenRead($zip)
        try {
            $entry = $reader.Entries | Where-Object { $_.FullName -eq $bin } | Select-Object -First 1
            if (-not $entry) { throw "$archive does not contain $bin" }
            [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $staged, $true)
        } finally { $reader.Dispose() }

        New-Item -ItemType Directory -Force -Path $installDir | Out-Null
        $target = Join-Path $installDir $bin
        $next = "$target.new"
        Copy-Item -LiteralPath $staged -Destination $next -Force
        try { Move-Item -LiteralPath $next -Destination $target -Force }
        catch { Remove-Item -LiteralPath $next -Force -ErrorAction SilentlyContinue; throw "cannot replace $target (is qbit-jay running?): $($_.Exception.Message)" }
        Write-Host "Installed $target"
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    if ($env:QBIT_JAY_NO_MODIFY_PATH -ne '1') {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $entries = @(); if ($userPath) { $entries = $userPath.Split(';') | Where-Object { $_ } }
        $normal = $installDir.TrimEnd('\')
        if (-not ($entries | Where-Object { $_.TrimEnd('\') -ieq $normal })) {
            $newPath = (@($entries) + $installDir) -join ';'
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
            Write-Host "Added $installDir to your user PATH. Open a new terminal to use it."
        }
        if (-not (($env:Path.Split(';')) | Where-Object { $_.TrimEnd('\') -ieq $normal })) { $env:Path = "$env:Path;$installDir" }
    } else {
        Write-Host "PATH left unchanged. Add $installDir to your PATH to run qbit-jay."
    }

    & (Join-Path $installDir $bin) version
}

try {
    Install-QbitJay
} catch {
    [Console]::Error.WriteLine("qbit-jay install: $($_.Exception.Message)")
    # Exiting would close the caller's window when this is piped into iex.
    if ($MyInvocation.MyCommand.Path) { exit 1 }
}
