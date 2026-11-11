# Totals a plan's token-usage entries per role. Reads the hook's JSON on stdin.
# Ends with one JSON line: status, summary, and details_path.
$ErrorActionPreference = 'Stop'

function Finish([string]$Status, [string]$Summary, [string]$Details) {
	$out = [ordered]@{ status = $Status; summary = $Summary }
	if ($Details) { $out.details_path = $Details }
	[Console]::Out.WriteLine(($out | ConvertTo-Json -Compress))
	exit 0
}

function Fail([string]$Message) {
	[Console]::Error.WriteLine($Message)
	exit 1
}

$text = [Console]::In.ReadToEnd()
$hook = $text | ConvertFrom-Json
$root = $hook.workspace_root
if (-not $root) { Fail 'no workspace_root in the hook input' }
if (-not $hook.record) { Finish 'skipped' 'no record at this point, so there is no plan to total' }
$record = [string]$hook.record.id
if ($record -notmatch '^[a-z0-9-]+$') { Fail "record id $record is not a plain ID" }

$folder = $null
if ($hook.extension.settings) { $folder = [string]$hook.extension.settings.folder }
if (-not $folder) { $folder = '.context-circuit/local/token-usage' }
if ($folder -match '^(/|\\|[A-Za-z]:)') { Fail "folder setting $folder is absolute; it must be relative to the workspace root" }
if (("/$folder/" -replace '\\', '/') -match '/\.\./') { Fail "folder setting $folder leaves the workspace" }

$dir = "$root/$folder"
$log = "$dir/$record.log"
if (-not (Test-Path -LiteralPath $log -PathType Leaf)) { Finish 'skipped' "no entries recorded for $record" }
$details = "$dir/$record-summary.md"

$entries = @{}; $sum = @{}; $na = @{}
$total = 0; $all = 0; $nna = 0; $bad = @()
$n = 0
foreach ($line in [System.IO.File]::ReadAllLines($log)) {
	$n++
	$line = $line.TrimEnd("`r")
	if ($line -match '^[ \t]*$' -or $line -match '^[ \t]*#') { continue }
	$role = ''; $tokens = ''
	foreach ($f in ($line -split '[ \t]+')) {
		$eq = $f.IndexOf('=')
		if ($eq -lt 1) { continue }
		$k = $f.Substring(0, $eq); $v = $f.Substring($eq + 1)
		if ($k -ceq 'role') { $role = $v } elseif ($k -ceq 'tokens') { $tokens = $v }
	}
	if (-not $role -or -not ($tokens -ceq 'not-available' -or $tokens -match '^[0-9]+$')) { $bad += $n; continue }
	if (-not $entries.ContainsKey($role)) { $entries[$role] = 0; $sum[$role] = [int64]0; $na[$role] = 0 }
	$entries[$role]++; $total++
	if ($tokens -ceq 'not-available') { $na[$role]++; $nna++ } else { $sum[$role] += [int64]$tokens; $all += [int64]$tokens }
}

$roles = [string[]]$entries.Keys
[Array]::Sort($roles, [StringComparer]::Ordinal)
$md = New-Object System.Collections.Generic.List[string]
$md.Add("# Token usage for $record"); $md.Add('')
$md.Add('| Role | Entries | Tokens | Not available |'); $md.Add('| --- | ---: | ---: | ---: |')
$parts = @()
foreach ($r in $roles) {
	$md.Add("| $r | $($entries[$r]) | $($sum[$r]) | $($na[$r]) |")
	$parts += "$r $($sum[$r])"
}
$md.Add("| Total | $total | $all | $nna |"); $md.Add('')
$md.Add("These totals cover only the $total entries recorded. An entry the agent did not write is not counted, and a not-available entry adds no tokens.")
$badList = $bad -join ', '
if ($bad.Count -gt 0) { $md.Add(''); $md.Add("Lines not read: $badList.") }
[System.IO.File]::WriteAllText($details, (($md -join "`n") + "`n"))

$s = "$total entries, $nna not-available"
if ($parts.Count -gt 0) { $s += ': ' + ($parts -join ', ') }
if ($bad.Count -gt 0) { $s += "; lines not read: $badList" }
$s += '; totals cover only the entries recorded'
Finish 'ok' $s $details
