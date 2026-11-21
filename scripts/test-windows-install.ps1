# End-to-end check of the Windows installer and service, run by the
# windows-service job in ci.yml on an elevated windows-latest runner.
#
#   scripts/test-windows-install.ps1 -ZipDir <dir laid out like the release zip>
#
# Fails on the first broken assertion or non-zero exit.
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$ZipDir
)

$ErrorActionPreference = "Stop"

$Installer = Join-Path $ZipDir "install.ps1"
$InstallDir = Join-Path $env:ProgramFiles "Trellis"
$Daemon = Join-Path $InstallDir "trellisd.exe"
$DataRoot = Join-Path $env:ProgramData "Trellis"
$DataDir = Join-Path $DataRoot "data"
$LogFile = Join-Path $DataRoot "logs\trellisd.log"
$Url = "http://127.0.0.1:8446"

function Assert-That([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw "ASSERTION FAILED: $Message" }
    Write-Host "ok: $Message"
}

# install.ps1 runs under Windows PowerShell 5.1, the shell an operator has.
function Invoke-Installer([string[]]$Arguments) {
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $Installer @Arguments
    return $LASTEXITCODE
}

function Show-ServiceLog {
    if (Test-Path $LogFile) { Get-Content $LogFile -Tail 50 }
    Get-WinEvent -FilterHashtable @{ LogName = "System"; ProviderName = "Service Control Manager" } -MaxEvents 5 -ErrorAction SilentlyContinue |
        Format-List TimeCreated, Message
}

# --- 1. A squatter created the data directory first -----------------------------------
# The runner's admin token owns what it creates as Administrators, so make the
# directory look the way a standard user's would: owned by a user, writable by
# Users, holding a file they planted.
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
$Planted = Join-Path $DataDir "planted.db"
Set-Content -Path $Planted -Value "squatter"
& icacls.exe $DataRoot /setowner $env:USERNAME /T /C | Out-Null
& icacls.exe $DataRoot /grant "*S-1-5-32-545:(OI)(CI)(M)" /T /C | Out-Null
Assert-That ((Get-Acl $DataDir).Owner -notlike "*Administrators") "precondition: the data dir starts user-owned"

Assert-That ((Invoke-Installer @()) -eq 0) "install.ps1 succeeds over a pre-created data dir"

# --- 2. Service shape -----------------------------------------------------------------
$Service = Get-CimInstance Win32_Service -Filter "Name='Trellis'"
Assert-That ($null -ne $Service) "service Trellis exists"
Assert-That ($Service.StartName -eq "NT SERVICE\Trellis") "service runs as NT SERVICE\Trellis (got $($Service.StartName))"
Assert-That ($Service.StartMode -eq "Manual") "service is manual start (got $($Service.StartMode))"
Assert-That ($Service.State -eq "Stopped") "service is not started by the install (got $($Service.State))"

# --- 3. The squatted directory was re-owned and locked down ---------------------------
$Allowed = @("NT AUTHORITY\SYSTEM", "BUILTIN\Administrators", "NT SERVICE\Trellis")
foreach ($Path in @($DataRoot, $DataDir, $Planted)) {
    $Acl = Get-Acl $Path
    Assert-That ($Acl.Owner -eq "BUILTIN\Administrators") "$Path is owned by Administrators (got $($Acl.Owner))"
    $Identities = @($Acl.Access | ForEach-Object { $_.IdentityReference.Value } | Select-Object -Unique)
    $Extra = @($Identities | Where-Object { $Allowed -notcontains $_ })
    Assert-That ($Extra.Count -eq 0) "$Path grants only SYSTEM, Administrators and the service (extra: $($Extra -join ', '))"
}
Assert-That ((Get-Acl $DataDir).AreAccessRulesProtected) "the data dir does not inherit from its parent"

# --- 4. A junction makes the installer refuse -----------------------------------------
$Target = Join-Path $env:RUNNER_TEMP "junction-target"
$Junction = Join-Path $DataDir "junction"
New-Item -ItemType Directory -Force -Path $Target | Out-Null
New-Item -ItemType Junction -Path $Junction -Target $Target | Out-Null
$Code = Invoke-Installer @()
Assert-That ($Code -ne 0) "install.ps1 refuses a data dir containing a junction (exit $Code)"
& cmd.exe /c rmdir $Junction
Assert-That (-not (Test-Path $Junction)) "the test junction was removed"

# --- 5. Run it ------------------------------------------------------------------------
& $Daemon service start
if ($LASTEXITCODE -ne 0) { Show-ServiceLog; throw "trellisd service start exited $LASTEXITCODE" }

$Version = $null
for ($i = 0; $i -lt 30 -and $null -eq $Version; $i++) {
    try {
        $Version = Invoke-RestMethod -Uri "$Url/__version" -TimeoutSec 2
    } catch {
        Start-Sleep -Seconds 1
    }
}
if ($null -eq $Version) { Show-ServiceLog; throw "$Url/__version never answered" }
Assert-That (-not [string]::IsNullOrEmpty($Version.version)) "/__version reports a version ($($Version.version))"
Assert-That (-not [string]::IsNullOrEmpty($Version.uiBuildHash) -and $Version.uiBuildHash -ne "unknown") "/__version reports an embedded UI hash"

$Good = & curl.exe -s -o NUL -w "%{http_code}" -H "Host: 127.0.0.1:8446" "$Url/__version"
Assert-That ($Good -eq "200") "a loopback Host header is served (got $Good)"
$Evil = & curl.exe -s -o NUL -w "%{http_code}" -H "Host: evil.example" "$Url/__version"
Assert-That ($Evil -eq "403") "Host: evil.example is refused with 403 (got $Evil)"
$Rebound = & curl.exe -s -o NUL -w "%{http_code}" -H "Host: evil.example:8446" "$Url/__version"
Assert-That ($Rebound -eq "403") "Host: evil.example:8446 is refused with 403 (got $Rebound)"

# --- 6. Stop and uninstall ------------------------------------------------------------
& $Daemon service stop
if ($LASTEXITCODE -ne 0) { Show-ServiceLog; throw "trellisd service stop exited $LASTEXITCODE" }
Assert-That ((Get-Service Trellis).Status -eq "Stopped") "service stopped"

Assert-That ((Invoke-Installer @("-Uninstall")) -eq 0) "install.ps1 -Uninstall succeeds"
Assert-That ($null -eq (Get-Service Trellis -ErrorAction SilentlyContinue)) "service removed"
Assert-That (-not (Test-Path $InstallDir)) "program files removed"
Assert-That (Test-Path $DataDir) "surveys directory kept"
