param([Parameter(Mandatory = $true)][long] $Window)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes
$root = [System.Windows.Automation.AutomationElement]::FromHandle([IntPtr] $Window)
$elements = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.Condition]::TrueCondition)
$nodes = @(foreach ($element in $elements) {
    $item = $element.Current
    $bounds = $item.BoundingRectangle
    if ($item.ClassName -eq 'VivaElement' -and $item.Name -and !$item.IsOffscreen -and !$bounds.IsEmpty) {
        @{ name = $item.Name; left = $bounds.Left; top = $bounds.Top; right = $bounds.Right; bottom = $bounds.Bottom }
    }
})
ConvertTo-Json -InputObject $nodes -Compress
