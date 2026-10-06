$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$destination = Join-Path ([IO.Path]::GetTempPath()) ('Ferrie test ' + [guid]::NewGuid())
$oldUserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$oldPath = $env:Path
try {
    & (Join-Path $root 'install.ps1') -Local -InstallDir $destination
    $version = @(& (Join-Path $destination 'ferrie.exe') --version)
    if ($LASTEXITCODE -ne 0 -or $version.Count -ne 2 -or $version[0] -ne 'ferrie version 0.2.2 (2026-10-06)' -or $version[1] -ne 'https://github.com/leo1394/homebrew-ferrie') { throw 'Windows launcher failed' }
    . (Join-Path $destination 'ferrie-completion.ps1')
    $result = [System.Management.Automation.CommandCompletion]::CompleteInput('ferrie --tar', 11, $null)
    if ('--target' -notin $result.CompletionMatches.CompletionText) { throw 'Option completion failed' }
    $package = Join-Path $destination 'app with spaces.apk'
    New-Item -ItemType File -Path $package | Out-Null
    $line = "ferrie --target '" + $destination + "\app"
    $result = [System.Management.Automation.CommandCompletion]::CompleteInput($line, $line.Length, $null)
    if (-not ($result.CompletionMatches.CompletionText -match 'app with spaces.apk')) { throw 'Path completion failed' }
    Write-Host 'Windows installer, launcher and completion passed'
} finally {
    [Environment]::SetEnvironmentVariable('Path', $oldUserPath, 'User')
    $env:Path = $oldPath
    if (Test-Path $destination) { Remove-Item -LiteralPath $destination -Force -Recurse }
}
