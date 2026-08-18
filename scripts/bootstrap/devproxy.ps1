$ErrorActionPreference = "Stop"

$BootstrapChannel = "v1"
$BaseUrl = $env:DEVPROXY_DOWNLOAD_BASE_URL
if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    $BaseUrl = "https://github.com/phy0hk/devproxy/releases/download/$BootstrapChannel"
}
$BaseUrl = $BaseUrl.TrimEnd("/")

$CacheDir = ".devproxy"
$ChannelDir = Join-Path $CacheDir $BootstrapChannel
$BinDir = Join-Path $ChannelDir "bin"
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
$Asset = "devproxy-windows-$Arch.exe"
$Bin = Join-Path $BinDir $Asset

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

if (!(Test-Path ".gitignore")) {
    Set-Content -Path ".gitignore" -Value ".devproxy/"
} else {
    $Gitignore = Get-Content ".gitignore"
    if (($Gitignore -notcontains ".devproxy/") -and ($Gitignore -notcontains ".devproxy")) {
        Add-Content -Path ".gitignore" -Value ".devproxy/"
    }
}

$ShouldDownload = $false
$Expected = $null

if ($env:DEVPROXY_SKIP_CHECKSUM -ne "1") {
    $Checksums = Join-Path $ChannelDir "checksums.txt"
    Invoke-WebRequest -Uri "$BaseUrl/checksums.txt" -OutFile $Checksums
    foreach ($Line in Get-Content $Checksums) {
        $Parts = $Line -split "\s+"
        if ($Parts.Length -ge 2) {
            $Name = $Parts[1].TrimStart("*")
            if ([System.IO.Path]::GetFileName($Name) -eq $Asset) {
                $Expected = $Parts[0]
                break
            }
        }
    }

    if ([string]::IsNullOrWhiteSpace($Expected)) {
        throw "no checksum found for $Asset"
    }
}

if (!(Test-Path $Bin)) {
    $ShouldDownload = $true
} elseif ($env:DEVPROXY_SKIP_CHECKSUM -ne "1") {
    $Actual = (Get-FileHash -Algorithm SHA256 $Bin).Hash.ToLowerInvariant()
    if ($Actual -ne $Expected.ToLowerInvariant()) {
        Write-Host "devproxy $BootstrapChannel binary is outdated, updating"
        $ShouldDownload = $true
    }
}

if ($ShouldDownload) {
    $Tmp = "$Bin.tmp"
    Write-Host "downloading $BaseUrl/$Asset"
    Invoke-WebRequest -Uri "$BaseUrl/$Asset" -OutFile $Tmp

    if ($env:DEVPROXY_SKIP_CHECKSUM -ne "1") {
        $Actual = (Get-FileHash -Algorithm SHA256 $Tmp).Hash.ToLowerInvariant()
        if ($Actual -ne $Expected.ToLowerInvariant()) {
            Remove-Item -Force $Tmp
            throw "checksum mismatch for $Asset"
        }
    } else {
        Write-Warning "checksum verification skipped"
    }

    Move-Item -Force $Tmp $Bin
}

& $Bin @args
exit $LASTEXITCODE
