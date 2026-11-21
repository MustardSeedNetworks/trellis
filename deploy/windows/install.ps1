# Trellis - Windows installer.
# Run as Administrator from an extracted Trellis Windows archive.
#
#   .\install.ps1              install, or upgrade in place
#   .\install.ps1 -Uninstall   remove the service and the program files
#
# Installs trellisd.exe and registers a Windows service named Trellis. Nothing
# is started and nothing starts at boot: the service is the operator's opt-in
# (`trellisd service start`), as the systemd user unit is on Linux (ADR-0007).
# Surveys are never deleted; they live under %ProgramData%\Trellis\data.
#
# The service runs as the virtual account NT SERVICE\Trellis, never as
# LocalSystem. It serves plain HTTP on loopback without a login, so anything
# that reaches that port must not land in a machine account. The installer
# refuses a TRELLIS_AUTH_* credential in the shell it runs from: a service's
# environment is readable by every local user.
#
# The service cannot scan Wi-Fi. Windows gates WLAN scanning on a per-user
# Location grant in an interactive session, which a service never has (#152).
# It serves imported surveys and reports; run trellisd.exe from your own
# session to survey with a live adapter.
[CmdletBinding()]
param(
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$InstallDir = Join-Path $env:ProgramFiles "Trellis"
$BinaryName = "trellisd.exe"
$ServiceName = "Trellis"
$SourceBinary = Join-Path $ScriptDir $BinaryName
$InstalledBinary = Join-Path $InstallDir $BinaryName
$DataRoot = Join-Path $env:ProgramData "Trellis"
$DataDir = if ($env:TRELLIS_DATA_DIR) { $env:TRELLIS_DATA_DIR } else { Join-Path $DataRoot "data" }

$Principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $Principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "Trellis installation requires an elevated PowerShell session. Reopen PowerShell as Administrator and run install.ps1 again."
    exit 1
}

function Stop-TrellisService {
    $Service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($null -ne $Service -and $Service.Status -ne "Stopped") {
        Write-Host "  Stopping Windows service..."
        Stop-Service -Name $ServiceName -Force
        $Service.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(30))
        return $true
    }
    return $false
}

function Remove-InstallDirFromPath {
    $MachinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $Entries = @($MachinePath -split ";" | Where-Object { $_ -and ($_.TrimEnd("\") -ne $InstallDir) })
    [Environment]::SetEnvironmentVariable("Path", ($Entries -join ";"), "Machine")
}

if ($Uninstall) {
    Write-Host "Uninstalling Trellis..." -ForegroundColor Green
    if ($null -ne (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) {
        [void](Stop-TrellisService)
        & $InstalledBinary service uninstall
        if ($LASTEXITCODE -ne 0) {
            $ExitCode = $LASTEXITCODE
            Write-Error "Trellis service removal failed with exit code $ExitCode."
            exit $ExitCode
        }
    }
    Remove-InstallDirFromPath
    if (Test-Path $InstallDir) {
        Remove-Item -Recurse -Force $InstallDir
    }
    Write-Host ""
    Write-Host "Trellis removed. Surveys in $DataDir were left in place; delete the folder to remove them." -ForegroundColor Green
    exit 0
}

if (-not (Test-Path $SourceBinary)) {
    Write-Error "Cannot find $BinaryName next to install.ps1. Extract the complete archive before installing."
    exit 1
}

Write-Host "Installing Trellis..." -ForegroundColor Green

$WasRunning = Stop-TrellisService

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item $SourceBinary $InstalledBinary -Force

$MachinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
if (@($MachinePath -split ";") -notcontains $InstallDir) {
    [Environment]::SetEnvironmentVariable("Path", "$MachinePath;$InstallDir", "Machine")
    Write-Host "  Added $InstallDir to system PATH"
}

if ($null -eq (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) {
    Write-Host "  Installing Windows service..."
    & $InstalledBinary service install
    if ($LASTEXITCODE -ne 0) {
        $ExitCode = $LASTEXITCODE
        Write-Error "Trellis service installation failed with exit code $ExitCode."
        exit $ExitCode
    }
}

# The service runs as the virtual account NT SERVICE\Trellis, which exists once
# the service does, so the directories are secured after it is registered and
# before it can be started.
#
# Anyone may create a folder under %ProgramData% before this script does. A
# directory they created and still own would let them rewrite what a machine
# account reads, so on every run (not only on create) a reparse point anywhere
# in the tree is refused, ownership of the whole tree goes to Administrators,
# and the directory gets a protected ACL: SYSTEM, Administrators and the
# service only. Children inherit it, so a file the squatter planted loses its
# own ACL too. Surveys record which networks were in range, so ordinary users
# get no access.
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $DataRoot "logs") | Out-Null
foreach ($Dir in @($DataRoot, $DataDir) | Select-Object -Unique) {
    $Item = Get-Item -LiteralPath $Dir -Force
    $Nested = Get-ChildItem -LiteralPath $Dir -Recurse -Force -Attributes ReparsePoint -ErrorAction SilentlyContinue
    if (($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $Nested) {
        Write-Error "$Dir contains a junction or symbolic link. Remove it and run install.ps1 again."
        exit 1
    }
    & icacls.exe $Dir /setowner "*S-1-5-32-544" /T /C /L | Out-Null
    if ($LASTEXITCODE -ne 0) {
        $ExitCode = $LASTEXITCODE
        Write-Error "Could not take ownership of $Dir (icacls exit code $ExitCode)."
        exit $ExitCode
    }
    & icacls.exe $Dir /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)(F)" "*S-1-5-32-544:(OI)(CI)(F)" "NT SERVICE\${ServiceName}:(OI)(CI)(M)" | Out-Null
    if ($LASTEXITCODE -ne 0) {
        $ExitCode = $LASTEXITCODE
        Write-Error "Could not restrict $Dir to SYSTEM, Administrators and the Trellis service (icacls exit code $ExitCode)."
        exit $ExitCode
    }
    if (Get-ChildItem -LiteralPath $Dir -Force | Select-Object -First 1) {
        & icacls.exe "$Dir\*" /reset /T /C /L | Out-Null
        if ($LASTEXITCODE -ne 0) {
            $ExitCode = $LASTEXITCODE
            Write-Error "Could not reset the ACLs under $Dir (icacls exit code $ExitCode)."
            exit $ExitCode
        }
    }
}

if ($WasRunning) {
    # An upgrade keeps the service's configuration and environment as the
    # operator left them; only the program file changed.
    Write-Host "  Restarting Windows service..."
    & $InstalledBinary service start
    if ($LASTEXITCODE -ne 0) {
        $ExitCode = $LASTEXITCODE
        Write-Error "Trellis service startup failed with exit code $ExitCode."
        exit $ExitCode
    }
}

Write-Host ""
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "  Service: $ServiceName (manual start, not started)"
Write-Host "  Data:    $DataDir"
Write-Host ""
Write-Host "Commands (elevated):"
Write-Host "  trellisd service start      - Start the service, then open http://127.0.0.1:8446"
Write-Host "  trellisd service stop       - Stop the service"
Write-Host "  trellisd service status     - Check service status"
Write-Host "  sc config Trellis start= auto   - Start the service with Windows"
Write-Host ""
Write-Host "For a live Wi-Fi survey, run trellisd.exe from your own session instead;"
Write-Host "the service cannot hold the Location permission Windows requires to scan."
