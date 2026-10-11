[CmdletBinding()]
param(
    [ValidateSet('Check','Install','Restore')][string]$Mode = 'Check',
    [string]$ClientPath,
    [string]$BackupPath
)
$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $repositoryRoot
$packageRoot = Join-Path $repositoryRoot 'client-patchs\japanese-voice'
function Hash([string]$Path) {
    $reader = [IO.File]::OpenRead($Path)
    $algorithm = [Security.Cryptography.SHA256]::Create()
    try { ([BitConverter]::ToString($algorithm.ComputeHash($reader))).Replace('-','').ToLowerInvariant() }
    finally { $algorithm.Dispose(); $reader.Dispose() }
}
function Save-Json($Value,[string]$Path) {
    $tempPath = $Path + '.tmp'
    [IO.File]::WriteAllText($tempPath,($Value | ConvertTo-Json -Depth 12),[Text.UTF8Encoding]::new($false))
    if ([IO.File]::Exists($Path)) { [IO.File]::Replace($tempPath,$Path,[NullString]::Value) }
    else { [IO.File]::Move($tempPath,$Path) }
}
function Assert-GameClosed {
    $games = @(Get-Process -Name DFO,DNF -ErrorAction SilentlyContinue)
    if ($games.Count) { throw ('游戏仍在运行，请正常退出后再安装或回退。进程：' + (($games | ForEach-Object { $_.Id }) -join ', ')) }
}
function Assert-Name([string]$Name) {
    if ($Name -notmatch '^sounds_[a-zA-Z0-9_]+\.npk$') { throw ('非法资源文件名：' + $Name) }
}
function Atomic-Copy([string]$Source,[string]$Target,[string]$Expected) {
    $temporary = $Target + '.' + [Guid]::NewGuid().ToString('N') + '.tmp'
    try {
        [IO.File]::Copy($Source,$temporary,$false)
        if ((Hash $temporary) -ne $Expected) { throw ('临时文件校验失败：' + $Target) }
        [IO.File]::Replace($temporary,$Target,[NullString]::Value)
        if ((Hash $Target) -ne $Expected) { throw ('替换后校验失败：' + $Target) }
    } finally {
        if ([IO.File]::Exists($temporary)) { [IO.File]::Delete($temporary) }
    }
}
$mutexFile = $null
$stagedFiles = @()
try {
    $manifest = Get-Content -LiteralPath (Join-Path $packageRoot 'manifest.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($manifest.schema -ne 1) { throw '不支持的补丁清单版本。' }
    if ($Mode -eq 'Restore') {
        if (-not $BackupPath) {
            $pointer = Join-Path $packageRoot 'last-install.json'
            if (-not (Test-Path -LiteralPath $pointer -PathType Leaf)) { throw '此补丁没有安装记录。' }
            $BackupPath = (Get-Content -LiteralPath $pointer -Raw -Encoding UTF8 | ConvertFrom-Json).backup_path
        }
        $BackupPath = [IO.Path]::GetFullPath($BackupPath)
        $journalPath = Join-Path $BackupPath 'installation.json'
        $journal = Get-Content -LiteralPath $journalPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($journal.schema -ne 1) { throw '不支持的备份记录版本。' }
        if (-not $ClientPath) { $ClientPath = $journal.client_path }
        if ([IO.Path]::GetFullPath($ClientPath).TrimEnd('\') -ne [IO.Path]::GetFullPath($journal.client_path).TrimEnd('\')) { throw '回退目标与备份记录不一致。' }
    } elseif (-not $ClientPath) { throw '请用 -ClientPath 指定客户端目录；不会自动选择正在测试的客户端。' }
    $ClientPath = [IO.Path]::GetFullPath($ClientPath).TrimEnd('\')
    $soundRoot = Join-Path $ClientPath 'SoundPacks'
    if (-not (Test-Path -LiteralPath (Join-Path $ClientPath 'DFO.exe') -PathType Leaf) -or -not (Test-Path -LiteralPath $soundRoot -PathType Container)) { throw ('不是有效的115us客户端目录：' + $ClientPath) }
    if ($Mode -eq 'Restore') {
        Assert-GameClosed
        $mutexFile = [IO.File]::Open((Join-Path $ClientPath '.115us-jp-voice.lock'),[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
        $pending = @()
        foreach ($row in $journal.files) {
            Assert-Name $row.name
            $target = Join-Path $soundRoot $row.name
            $original = Join-Path (Join-Path $BackupPath 'originals') $row.name
            if ((Hash $original) -ne $row.original_sha256) { throw ('备份校验失败：' + $row.name) }
            $current = Hash $target
            if ($current -eq $row.original_sha256) { continue }
            if ($current -ne $row.patched_sha256) { throw ('文件已被其他补丁修改，停止回退：' + $row.name) }
            $pending += $row
        }
        foreach ($row in $pending) {
            Assert-GameClosed
            $target = Join-Path $soundRoot $row.name
            if ((Hash $target) -ne $row.patched_sha256) { throw ('回退前文件已变化：' + $row.name) }
            Atomic-Copy (Join-Path (Join-Path $BackupPath 'originals') $row.name) $target $row.original_sha256
        }
        $journal.status = 'restored'
        Save-Json $journal $journalPath
        Write-Host ('已恢复原语音，文件数：' + $pending.Count)
        exit 0
    }
    $unknown = @(Get-ChildItem -LiteralPath $soundRoot -Filter '*.npk' -File | Where-Object { $_.Name -notin $manifest.baseline_pack_names })
    if ($unknown.Count) { throw ('发现其他语音补丁，请先单独处理，避免互相覆盖：' + (($unknown.Name) -join ', ')) }
    $pending = @()
    foreach ($row in $manifest.files) {
        Assert-Name $row.name
        $payload = Join-Path (Join-Path $packageRoot 'payload\SoundPacks') $row.name
        if ((Hash $payload) -ne $row.patched_sha256) { throw ('补丁文件损坏：' + $row.name) }
        $target = Join-Path $soundRoot $row.name
        $current = Hash $target
        if ($current -eq $row.patched_sha256) { continue }
        if ($current -ne $row.original_sha256) { throw ('原语音版本不匹配，停止覆盖：' + $row.name) }
        $pending += $row
    }
    Write-Host ('校验通过。客户端：' + $ClientPath)
    Write-Host ('补丁资源包：' + $manifest.files.Count + '，待安装：' + $pending.Count + '，音频变化：' + $manifest.changed_entries)
    if ($Mode -eq 'Check') { Write-Host '只读检查完成，客户端未修改。'; exit 0 }
    Assert-GameClosed
    if (-not $pending.Count) { Write-Host '此客户端已经安装日服语音补丁。'; exit 0 }
    if ($pending.Count -ne $manifest.files.Count) { throw '检测到部分安装状态，请先运行回退入口恢复原语音，再重新安装。' }
    $mutexFile = [IO.File]::Open((Join-Path $ClientPath '.115us-jp-voice.lock'),[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
    if (-not $BackupPath) { $BackupPath = Join-Path (Join-Path $packageRoot 'backups') ((Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,8)) }
    $BackupPath = [IO.Path]::GetFullPath($BackupPath)
    if (Test-Path -LiteralPath $BackupPath) { throw '备份目录已存在，请使用新目录。' }
    $originalRoot = Join-Path $BackupPath 'originals'
    $null = New-Item -ItemType Directory -Path $originalRoot
    $journalPath = Join-Path $BackupPath 'installation.json'
    $journal = [pscustomobject]@{ schema=1; client_path=$ClientPath; backup_path=$BackupPath; status='preparing'; files=@($pending) }
    Save-Json $journal $journalPath
    foreach ($row in $pending) {
        Assert-GameClosed
        $target = Join-Path $soundRoot $row.name
        if ((Hash $target) -ne $row.original_sha256) { throw ('安装前文件已变化：' + $row.name) }
        $original = Join-Path $originalRoot $row.name
        [IO.File]::Copy($target,$original,$false)
        if ((Hash $original) -ne $row.original_sha256) { throw ('备份失败：' + $row.name) }
        $staged = $target + '.' + [Guid]::NewGuid().ToString('N') + '.tmp'
        $stagedFiles += $staged
        [IO.File]::Copy((Join-Path (Join-Path $packageRoot 'payload\SoundPacks') $row.name),$staged,$false)
        if ((Hash $staged) -ne $row.patched_sha256) { throw ('补丁暂存失败：' + $row.name) }
    }
    $journal.status = 'prepared'
    Save-Json $journal $journalPath
    Save-Json ([pscustomobject]@{ backup_path=$BackupPath; client_path=$ClientPath }) (Join-Path $packageRoot 'last-install.json')
    $committed = @()
    try {
        for ($i=0; $i -lt $pending.Count; $i++) {
            Assert-GameClosed
            $row = $pending[$i]; $target = Join-Path $soundRoot $row.name
            if ((Hash $target) -ne $row.original_sha256) { throw ('写入前文件已变化：' + $row.name) }
            [IO.File]::Replace($stagedFiles[$i],$target,[NullString]::Value)
            $committed += $row
            if ((Hash $target) -ne $row.patched_sha256) { throw ('安装后校验失败：' + $row.name) }
        }
        $journal.status = 'installed'
        Save-Json $journal $journalPath
    } catch {
        $failure = $_
        $rollbackFailures = @()
        foreach ($row in $committed) {
            try {
                $target = Join-Path $soundRoot $row.name
                if ((Hash $target) -ne $row.patched_sha256) { throw '目标又被修改，保留现场' }
                Atomic-Copy (Join-Path $originalRoot $row.name) $target $row.original_sha256
            } catch { $rollbackFailures += ($row.name + ': ' + $_.Exception.Message) }
        }
        $journal.status = 'installation_failed'
        Save-Json $journal $journalPath
        if ($rollbackFailures.Count) { Write-Host ('部分文件需从备份恢复：' + ($rollbackFailures -join '; ')) }
        throw $failure
    }
    Write-Host ('日服语音已安装。备份：' + $BackupPath)
    Write-Host '请手动启动游戏试听。回退请运行 scripts\jp-voice-restore.cmd。'
    exit 0
} catch {
    Write-Host ('失败：' + $_.Exception.Message) -ForegroundColor Red
    exit 1
} finally {
    foreach ($staged in $stagedFiles) { if ([IO.File]::Exists($staged)) { [IO.File]::Delete($staged) } }
    if ($mutexFile) { $mutexFile.Dispose() }
}
