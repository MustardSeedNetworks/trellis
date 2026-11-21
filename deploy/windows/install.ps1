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
# Anyone may create a folder under %ProgramData% before this script does, so an
# existing directory is trusted only if it already looks like ours, and is never
# repaired in place:
#   - missing: created with its complete security descriptor in one call, so it
#     never exists with an inherited ACL;
#   - present, owned by SYSTEM or Administrators, with nothing under it owned by
#     anyone else and no reparse point anywhere: its DACL is replaced wholesale,
#     not added to, and read back;
#   - anything else: refused, with the path, for an administrator to inspect and
#     remove. No recursive icacls over a tree someone else may have built.
$SystemSid = New-Object Security.Principal.SecurityIdentifier("S-1-5-18")
$AdminsSid = New-Object Security.Principal.SecurityIdentifier("S-1-5-32-544")
$ServiceSid = (New-Object Security.Principal.NTAccount("NT SERVICE\$ServiceName")).Translate([Security.Principal.SecurityIdentifier])
$Expected = @{}
$Expected[$SystemSid.Value] = [Security.AccessControl.FileSystemRights]::FullControl
$Expected[$AdminsSid.Value] = [Security.AccessControl.FileSystemRights]::FullControl
$Expected[$ServiceSid.Value] = [Security.AccessControl.FileSystemRights]::Modify

function Get-TrustedDescriptor {
    $Security = New-Object Security.AccessControl.DirectorySecurity
    $Security.SetAccessRuleProtection($true, $false)
    $Security.SetOwner($AdminsSid)
    foreach ($Sid in @($SystemSid, $AdminsSid, $ServiceSid)) {
        $Rule = New-Object Security.AccessControl.FileSystemAccessRule($Sid, $Expected[$Sid.Value],
            "ContainerInherit, ObjectInherit", "None", "Allow")
        $Security.AddAccessRule($Rule)
    }
    return $Security
}

function Assert-TrustedDescriptor([string]$Path) {
    $Acl = Get-Acl -LiteralPath $Path
    $Rules = @($Acl.Access)
    $Granted = @($Rules | Where-Object {
        $Sid = $_.IdentityReference.Translate([Security.Principal.SecurityIdentifier]).Value
        $Expected.ContainsKey($Sid) -and $_.AccessControlType -eq "Allow" -and $_.FileSystemRights -eq $Expected[$Sid] -and -not $_.IsInherited
    })
    $Owner = $Acl.GetOwner([Security.Principal.SecurityIdentifier])
    if (-not $Acl.AreAccessRulesProtected -or $Rules.Count -ne 3 -or $Granted.Count -ne 3 -or $Owner -ne $AdminsSid) {
        Write-Error "$Path did not end with exactly SYSTEM, Administrators and the Trellis service. Inspect it and remove it, then run install.ps1 again."
        exit 1
    }
}

function Protect-Directory([string]$Path) {
    if (Test-Path -LiteralPath $Path) {
        $Item = Get-Item -LiteralPath $Path -Force
        $Nested = @(Get-ChildItem -LiteralPath $Path -Recurse -Force -Attributes ReparsePoint)
        if (-not $Item.PSIsContainer -or ($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $Nested.Count -gt 0) {
            Write-Error "$Path is not a plain directory or contains a junction or symbolic link. Inspect it and remove it, then run install.ps1 again."
            exit 1
        }
        $Trusted = @($SystemSid.Value, $AdminsSid.Value, $ServiceSid.Value)
        $Owners = @(@($Item) + @(Get-ChildItem -LiteralPath $Path -Recurse -Force) | ForEach-Object {
            (Get-Acl -LiteralPath $_.FullName).GetOwner([Security.Principal.SecurityIdentifier]).Value
        })
        $Untrusted = @($Owners | Where-Object { $Trusted -notcontains $_ })
        if ($Untrusted.Count -gt 0) {
            Write-Error "$Path, or something under it, is owned by $($Untrusted[0]), not SYSTEM, Administrators or the Trellis service. Inspect it and remove it, then run install.ps1 again."
            exit 1
        }
        Set-Acl -LiteralPath $Path -AclObject (Get-TrustedDescriptor)
    } else {
        [void][IO.Directory]::CreateDirectory($Path, (Get-TrustedDescriptor))
    }
    Assert-TrustedDescriptor $Path
}

Protect-Directory $DataRoot
if ($DataDir -ne $DataRoot) {
    Protect-Directory $DataDir
}
New-Item -ItemType Directory -Force -Path (Join-Path $DataRoot "logs") | Out-Null

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
