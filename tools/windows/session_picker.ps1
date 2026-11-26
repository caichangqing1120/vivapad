param(
    [Parameter(Mandatory = $true)][long] $Window,
    [Parameter(Mandatory = $true)][string] $SessionPath
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes
$dialog = [System.Windows.Automation.AutomationElement]::FromHandle([IntPtr] $Window)
if (-not $dialog -or $dialog.Current.Name -cne 'Load Session') {
    throw 'Expected the app-owned Load Session dialog'
}

$descendants = [System.Windows.Automation.TreeScope]::Descendants
$all = [System.Windows.Automation.Condition]::TrueCondition
$deadline = [DateTime]::UtcNow.AddSeconds(5)
do {
    $controls = $dialog.FindAll($descendants, $all)
    $edit = @(foreach ($element in $controls) {
        $item = $element.Current
        if ($item.IsEnabled -and $item.ControlType -eq [System.Windows.Automation.ControlType]::Edit -and
            ($item.AutomationId -eq '1001' -or $item.Name -eq 'File name:')) {
            $element
        }
    })
    $open = @(foreach ($element in $controls) {
        $item = $element.Current
        if ($item.IsEnabled -and $item.ControlType -eq [System.Windows.Automation.ControlType]::Button -and
            $item.AutomationId -eq '1' -and $item.Name -eq 'Open') {
            $element
        }
    })
    if ($edit.Count -eq 1 -and $open.Count -eq 1) { break }
    Start-Sleep -Milliseconds 100
} while ([DateTime]::UtcNow -lt $deadline)

if ($edit.Count -ne 1 -or $open.Count -ne 1) {
    throw "Expected one filename edit and one Open button; found $($edit.Count) and $($open.Count)"
}

$value = $edit[0].GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
$value.SetValue($SessionPath)
if ($value.Current.Value -cne $SessionPath) {
    throw 'The native filename field did not retain the full session path'
}
Write-Output 'Verified the full filename in the Windows file picker'
$invoke = $open[0].GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
$invoke.Invoke()
