# Installs the server build of Hyla on Windows for the current user, without
# administrator rights: hyla.exe in %LOCALAPPDATA%\Programs\hyla-server, on
# the user's PATH, and hylad as a task of the user's that starts as they log
# on, so that Hyla on another machine shows this one's sessions, over SSH
# (Settings > Hosts).
#
#   irm https://github.com/smallnest/hyla/releases/latest/download/install-server.ps1 | iex
#   & ([scriptblock]::Create((irm <that url>))) -Version 2.1.0 -NoService
#   ... -Uninstall
#
# The release workflow writes the version of its release in place of
# @VERSION@. $env:HYLA_RELEASES names a mirror of the releases, as hyla
# daemon update takes it. It is ASCII only, as Windows PowerShell reads a
# download in the system's code page.
param(
	[string]$Version = '@VERSION@',
	[switch]$NoService,
	[switch]$DryRun,
	[switch]$Uninstall
)

$repo = 'smallnest/hyla'
$lib = Join-Path $env:LOCALAPPDATA 'Programs\hyla-server'
$hyla = Join-Path $lib 'hyla.exe'
# The file that tells an install of this script's, which updates itself.
$marker = '.hyla-server'
# The task hyla daemon install-service registers, as it keeps it.
$task = Join-Path $env:APPDATA 'hyla\hylad.task.xml'

# Failures throw, which ends the script, but not the PowerShell it runs in,
# as exit would one that ran it with iex.
function Fail([string]$msg) { throw "install-server.ps1: $msg" }

function Fetch([string]$url, [string]$file) {
	[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
	$wc = New-Object Net.WebClient
	try {
		$wc.DownloadFile($url, $file)
	} catch {
		$e = if ($_.Exception.InnerException) { $_.Exception.InnerException } else { $_.Exception }
		Fail "could not download ${url}: $($e.Message)"
	} finally { $wc.Dispose() }
}

# The user's PATH, as the registry keeps it, its %VARIABLES% unexpanded.
function Get-UserPath {
	$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
	try { return [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames) } finally { $key.Close() }
}

function Set-UserPath([string[]]$parts) {
	$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
	try { $key.SetValue('Path', ($parts -join ';'), [Microsoft.Win32.RegistryValueKind]::ExpandString) } finally { $key.Close() }
	# Setting a variable tells running programs, as Explorer, that the
	# environment changed, for the terminals they start.
	[Environment]::SetEnvironmentVariable('HYLA_INSTALL', '1', 'User')
	[Environment]::SetEnvironmentVariable('HYLA_INSTALL', $null, 'User')
}

# Quiet runs hyla, saying nothing: Windows PowerShell would take what hyla
# writes on its standard error, sent away, for an error of its own, and stop.
function Quiet { $ErrorActionPreference = 'SilentlyContinue'; & $hyla @args 2>$null }

function Same-Dir([string]$a, [string]$b) { return $a.TrimEnd('\') -ieq $b.TrimEnd('\') }

# What an update moved aside, but what still runs: hyla's own names,
# hyla.exe.<stamp>.hyla-old, as hyla daemon update gives them.
function Remove-Old([string]$dir) {
	Get-ChildItem -LiteralPath $dir -Filter '*.hyla-old' -Force -ErrorAction SilentlyContinue |
		ForEach-Object { Remove-Item -LiteralPath $_.FullName -Recurse -Force -ErrorAction SilentlyContinue }
}

# The preferences are the functions' own: a run with iex would leave them
# in the user's PowerShell otherwise.
function Uninstall-Hyla {
	$ErrorActionPreference = 'Stop'
	if (Test-Path -LiteralPath $hyla) {
		Quiet daemon uninstall-service | Out-Null
		Quiet daemon stop | Out-Null
	}
	$parts = @((Get-UserPath) -split ';' | Where-Object { $_ -and -not (Same-Dir $_ $lib) })
	Set-UserPath $parts
	# hylad lets go of its files a moment after it stops.
	foreach ($i in 1..10) {
		Remove-Item -LiteralPath $lib -Recurse -Force -ErrorAction SilentlyContinue
		if (-not (Test-Path -LiteralPath $lib)) { break }
		Start-Sleep -Milliseconds 300
	}
	if (Test-Path -LiteralPath $lib) {
		Write-Output "Some of $lib is still running: remove it once Hyla there is done."
	} else {
		Write-Output "Removed Hyla from $lib. Your settings and data stay in $env:APPDATA\hyla."
	}
}

function Install-Hyla {
	$ErrorActionPreference = 'Stop'
	$ProgressPreference = 'SilentlyContinue'
	$arch = $null
	try {
		$arch = @{ 'X64' = 'amd64'; 'Arm64' = 'arm64' }[[Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()]
	} catch {
		$a = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
		$arch = @{ 'AMD64' = 'amd64'; 'ARM64' = 'arm64' }[$a]
	}
	if (-not $arch) { Fail 'Hyla is made for 64-bit Windows, on x64 or Arm' }

	$v = $Version.TrimStart('v')
	if ($v.StartsWith('@')) {
		# This script, run from the repository: the latest release.
		[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
		$v = ([string](Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/$repo/releases/latest").tag_name).TrimStart('v')
	}
	if ($v -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$') { Fail "$v is not a version of Hyla" }

	$name = "hyla-server-$v-windows-$arch.zip"
	$releases = if ($env:HYLA_RELEASES) { $env:HYLA_RELEASES.TrimEnd('/') } else { "https://github.com/$repo/releases/download" }
	$url = "$releases/v$v/$name"
	$work = Join-Path ([IO.Path]::GetTempPath()) ("hyla-install-" + [Guid]::NewGuid().ToString('N'))
	$staging = "$lib.new"
	New-Item -ItemType Directory -Force -Path $work | Out-Null
	try {
		Write-Output "Downloading Hyla $v for windows/$arch"
		$zip = Join-Path $work $name
		Fetch $url $zip
		Fetch "$url.sha256" "$zip.sha256"
		$want = ((Get-Content -LiteralPath "$zip.sha256" -TotalCount 1) -split '\s+')[0]
		if ((Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash -ine $want) { Fail "$name is not the release's: its SHA-256 differs" }

		Add-Type -AssemblyName System.IO.Compression.FileSystem
		$z = [IO.Compression.ZipFile]::OpenRead($zip)
		try {
			foreach ($e in $z.Entries) {
				$n = $e.FullName -replace '\\', '/'
				if ($n.StartsWith('/') -or $n -match '^[A-Za-z]:' -or $n -match '(^|/)\.\.(/|$)') { Fail "$name holds files outside it" }
			}
		} finally { $z.Dispose() }

		if ($DryRun) {
			Write-Output "would install Hyla $v in $lib, put it on your PATH, and run hylad as a task: $(-not $NoService)"
			return
		}

		# The install of 2.x, when Hyla was Kero: its task and its files
		# go, so that one hylad serves this machine. Its settings and
		# sessions stay: Hyla moves them to %APPDATA%\hyla as it starts.
		$oldlib = Join-Path $env:LOCALAPPDATA 'Programs\kero-server'
		$errorAction = $ErrorActionPreference
		$ErrorActionPreference = 'SilentlyContinue'
		if (Test-Path -LiteralPath (Join-Path $oldlib 'kero.exe')) {
			& (Join-Path $oldlib 'kero.exe') daemon uninstall-service *> $null
			& (Join-Path $oldlib 'kero.exe') daemon stop *> $null
			Remove-Item -LiteralPath $oldlib -Recurse -Force -ErrorAction SilentlyContinue
		}
		# No task of 2.x's is no error, which schtasks says on stderr: cmd
		# drops it, so that a caller's $ErrorActionPreference = 'Stop' holds.
		cmd /c 'schtasks /Delete /TN \Kero\kerod /F >nul 2>&1'
		$ErrorActionPreference = $errorAction

		$upgrade = Test-Path -LiteralPath $hyla
		Write-Output "Installing in $lib"
		Remove-Item -LiteralPath $staging -Recurse -Force -ErrorAction SilentlyContinue
		[IO.Compression.ZipFile]::ExtractToDirectory($zip, $staging)
		$staged = Join-Path $staging 'hyla.exe'
		if (-not (Test-Path -LiteralPath $staged)) { Fail "$name holds no hyla.exe" }
		& $staged version | Out-Null
		if ($LASTEXITCODE -ne 0) { Fail "the hyla.exe of $name does not run here" }
		New-Item -ItemType File -Force -Path (Join-Path $staging $marker) | Out-Null

		# File by file: what hylad runs, hyla.exe and its library, can be
		# moved but not replaced, nor can the directory that holds them.
		New-Item -ItemType Directory -Force -Path $lib | Out-Null
		Remove-Old $lib
		$stamp = [DateTime]::UtcNow.Ticks
		$moved = New-Object Collections.ArrayList
		try {
			foreach ($f in @(Get-ChildItem -LiteralPath $lib -Force | Where-Object { $_.Name -notlike '*.hyla-old' })) {
				$to = Join-Path $lib "$($f.Name).$stamp.hyla-old"
				Move-Item -LiteralPath $f.FullName -Destination $to
				[void]$moved.Add(@($f.FullName, $to))
			}
			foreach ($f in @(Get-ChildItem -LiteralPath $staging -Force)) {
				$to = Join-Path $lib $f.Name
				Move-Item -LiteralPath $f.FullName -Destination $to
				[void]$moved.Add(@($f.FullName, $to))
			}
		} catch {
			# Back as it was: the install of before, whole.
			for ($i = $moved.Count - 1; $i -ge 0; $i--) {
				Move-Item -LiteralPath $moved[$i][1] -Destination $moved[$i][0] -ErrorAction SilentlyContinue
			}
			Fail "could not replace the files of ${lib}: $($_.Exception.Message)"
		}
		Remove-Old $lib
	} finally {
		Remove-Item -LiteralPath $work, $staging -Recurse -Force -ErrorAction SilentlyContinue
	}

	$parts = @((Get-UserPath) -split ';' | Where-Object { $_ })
	if (-not ($parts | Where-Object { Same-Dir $_ $lib })) {
		Set-UserPath ($parts + $lib)
		Write-Output "Added $lib to your PATH, for the terminals and SSH connections that start from now on."
	}
	if (-not (($env:Path -split ';') | Where-Object { $_ -and (Same-Dir $_ $lib) })) { $env:Path = "$env:Path;$lib" }

	$running = $false
	if ($upgrade) {
		$running = [bool](Quiet daemon status | Select-String -Quiet '^hylad [0-9]+ running')
	}
	if ($running) {
		# Never a restart unasked: it would end the programs of the sessions.
		Write-Output 'hylad runs the version of before until it restarts: hyla daemon restart (its sessions start again, the programs in them end).'
	} elseif (Test-Path -LiteralPath $task) {
		# The task runs this install's hyla as it starts.
	} elseif (-not $NoService) {
		& $hyla daemon install-service
		if ($LASTEXITCODE -ne 0) { Write-Output 'hylad does not run as a task: Hyla starts it as it connects.' }
	} else {
		Write-Output 'hylad starts as Hyla connects. To start it as you log on: hyla daemon install-service'
	}

	Write-Output ''
	Write-Output "Hyla $v is installed. In Hyla on your computer, add this machine in Settings > Hosts:"
	Write-Output "  $env:USERNAME@$env:COMPUTERNAME"
	Write-Output "or from its command line: hyla hosts add $($env:COMPUTERNAME.ToLower()) $env:USERNAME@$env:COMPUTERNAME"
	if (-not (Get-Service -Name sshd -ErrorAction SilentlyContinue)) {
		Write-Output ''
		Write-Output 'Hyla reaches Windows over SSH: turn on OpenSSH Server in Settings > System > Optional features, as an administrator.'
	}
}

if ($Uninstall) { Uninstall-Hyla } else { Install-Hyla }
