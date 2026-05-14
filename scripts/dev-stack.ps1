param(
    [ValidateSet("start", "restart", "stop", "status")]
    [string]$Action = "restart",

    [string]$AIClientPath = "",

    [switch]$SkipAIClient,

    [int]$StartupTimeoutSeconds = 45
)

$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$StateDir = Join-Path $RepoRoot "tmp\dev-stack"
$LogDir = Join-Path $StateDir "logs"
$StateFile = Join-Path $StateDir "processes.json"

$BackendDir = Join-Path $RepoRoot "backend"
$FrontendDir = Join-Path $RepoRoot "frontend"
if ([string]::IsNullOrWhiteSpace($AIClientPath)) {
    $AIClientPath = Join-Path (Split-Path -Parent $RepoRoot) "AIClient2API"
}

New-Item -ItemType Directory -Force -Path $StateDir, $LogDir | Out-Null

function Write-Step {
    param([string]$Message)
    Write-Host "[dev-stack] $Message"
}

function Get-ProcessTreeIds {
    param([int]$RootProcessId)

    $ids = New-Object System.Collections.Generic.List[int]
    $queue = New-Object System.Collections.Generic.Queue[int]
    $queue.Enqueue($RootProcessId)

    while ($queue.Count -gt 0) {
        $current = $queue.Dequeue()
        if (-not $ids.Contains($current)) {
            $ids.Add($current)
            Get-CimInstance Win32_Process -Filter "ParentProcessId=$current" |
                ForEach-Object { $queue.Enqueue([int]$_.ProcessId) }
        }
    }

    return $ids.ToArray()
}

function Stop-ProcessTree {
    param([int]$ProcessId)

    $ids = Get-ProcessTreeIds -RootProcessId $ProcessId
    [array]::Reverse($ids)
    foreach ($id in $ids) {
        $process = Get-Process -Id $id -ErrorAction SilentlyContinue
        if ($null -ne $process) {
            Stop-Process -Id $id -Force -ErrorAction SilentlyContinue
        }
    }
}

function Read-State {
    if (-not (Test-Path $StateFile)) {
        return @()
    }

    $state = Get-Content -Raw -Path $StateFile | ConvertFrom-Json
    if ($null -eq $state) {
        return @()
    }
    if ($state -is [array]) {
        return $state
    }
    return @($state)
}

function Save-State {
    param([array]$Processes)

    $json = if ($Processes.Count -eq 0) {
        "[]"
    }
    else {
        $Processes | ConvertTo-Json -Depth 4
    }
    Set-Content -Path $StateFile -Value $json -Encoding UTF8
}

function Get-PortProcessIds {
    param([int]$Port)

    $connections = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    return @($connections | Select-Object -ExpandProperty OwningProcess -Unique)
}

function Stop-ManagedProcesses {
    $state = Read-State
    foreach ($entry in $state) {
        if ($entry.PID) {
            Write-Step "Stopping $($entry.Name) pid=$($entry.PID)"
            Stop-ProcessTree -ProcessId ([int]$entry.PID)
        }
    }
    Save-State -Processes @()
}

function Stop-PortProcesses {
    param([array]$Services)

    foreach ($service in $Services) {
        foreach ($port in $service.Ports) {
            $ids = Get-PortProcessIds -Port $port
            foreach ($id in $ids) {
                Write-Step "Stopping process on port $port for $($service.Name), pid=$id"
                Stop-ProcessTree -ProcessId ([int]$id)
            }
        }
    }
}

function Test-PortOpen {
    param(
        [string]$HostName,
        [int]$Port
    )

    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $async = $client.BeginConnect($HostName, $Port, $null, $null)
        if (-not $async.AsyncWaitHandle.WaitOne(500)) {
            return $false
        }
        $client.EndConnect($async)
        return $true
    }
    catch {
        return $false
    }
    finally {
        $client.Close()
    }
}

function Start-ServiceProcess {
    param(
        [string]$Name,
        [string]$WorkingDirectory,
        [string]$Command,
        [int]$Port,
        [int[]]$Ports = @($Port)
    )

    if (-not (Test-Path $WorkingDirectory)) {
        throw "$Name working directory does not exist: $WorkingDirectory"
    }

    $stdout = Join-Path $LogDir "$Name.out.log"
    $stderr = Join-Path $LogDir "$Name.err.log"
    Write-Step "Starting $Name on port $Port"
    $process = Start-Process `
        -FilePath "powershell.exe" `
        -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $Command) `
        -WorkingDirectory $WorkingDirectory `
        -RedirectStandardOutput $stdout `
        -RedirectStandardError $stderr `
        -WindowStyle Hidden `
        -PassThru

    return [pscustomobject]@{
        Name = $Name
        PID = $process.Id
        Ports = $Ports
        WorkingDirectory = $WorkingDirectory
        Command = $Command
        Stdout = $stdout
        Stderr = $stderr
        StartedAt = (Get-Date).ToString("s")
    }
}

function Show-Status {
    $state = Read-State
    if ($state.Count -eq 0) {
        Write-Step "No managed dev-stack processes are recorded."
    }

    foreach ($entry in $state) {
        $process = Get-Process -Id ([int]$entry.PID) -ErrorAction SilentlyContinue
        $stateText = if ($null -eq $process) { "stopped" } else { "running" }
        $ports = @($entry.Ports)
        if ($ports.Count -eq 0 -and $entry.Port) {
            $ports = @($entry.Port)
        }
        $portStates = foreach ($port in $ports) {
            $portText = if (Test-PortOpen -HostName "127.0.0.1" -Port ([int]$port)) { "listening" } else { "not listening" }
            "${port}:${portText}"
        }
        Write-Host ("{0,-12} pid={1,-8} ports={2,-28} {3}" -f $entry.Name, $entry.PID, ($portStates -join ", "), $stateText)
    }

    foreach ($port in @(5432, 6379)) {
        $label = if ($port -eq 5432) { "PostgreSQL" } else { "Redis" }
        $stateText = if (Test-PortOpen -HostName "127.0.0.1" -Port $port) { "ready" } else { "not reachable" }
        Write-Host ("{0,-12} port={1,-6} {2}" -f $label, $port, $stateText)
    }
}

function Wait-ServicePorts {
    param([array]$Services)

    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    Write-Step "Waiting up to $StartupTimeoutSeconds seconds for dev ports to listen"

    do {
        $pending = @($Services | Where-Object {
            -not (Test-PortOpen -HostName "127.0.0.1" -Port ([int]$_.Port))
        })
        if ($pending.Count -eq 0) {
            return
        }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)

    foreach ($service in $pending) {
        Write-Warning "$($service.Name) did not listen on 127.0.0.1:$($service.Port) within $StartupTimeoutSeconds seconds. Check logs under $LogDir."
    }
}

$services = @(
    [pscustomobject]@{
        Name = "backend"
        WorkingDirectory = $BackendDir
        Command = if (Test-Path "E:\sysware\GoProject\bin\air.exe") { "& 'E:\sysware\GoProject\bin\air.exe'" } else { "air" }
        Port = 18081
        Ports = @(18081)
    },
    [pscustomobject]@{
        Name = "frontend"
        WorkingDirectory = $FrontendDir
        Command = "pnpm dev"
        Port = 15174
        Ports = @(15174)
    }
)

if (-not $SkipAIClient) {
    $services += [pscustomobject]@{
        Name = "aiclient2api"
        WorkingDirectory = $AIClientPath
        Command = "pnpm start"
        Port = 3000
        Ports = @(3000, 3100)
    }
}

switch ($Action) {
    "status" {
        Show-Status
        break
    }
    "stop" {
        Stop-ManagedProcesses
        Stop-PortProcesses -Services $services
        Show-Status
        break
    }
    "restart" {
        Stop-ManagedProcesses
        Stop-PortProcesses -Services $services
    }
}

if ($Action -in @("start", "restart")) {
    foreach ($dependency in @(
        [pscustomobject]@{ Name = "PostgreSQL"; Port = 5432 },
        [pscustomobject]@{ Name = "Redis"; Port = 6379 }
    )) {
        if (-not (Test-PortOpen -HostName "127.0.0.1" -Port $dependency.Port)) {
            Write-Warning "$($dependency.Name) is not reachable on 127.0.0.1:$($dependency.Port). Start Docker Desktop/dev containers first if this service is required."
        }
    }

    $started = @()
    foreach ($service in $services) {
        $started += Start-ServiceProcess `
            -Name $service.Name `
            -WorkingDirectory $service.WorkingDirectory `
            -Command $service.Command `
            -Port $service.Port `
            -Ports $service.Ports
    }
    Save-State -Processes $started

    Wait-ServicePorts -Services $services
    Show-Status
    Write-Step "Logs are under $LogDir"
    Write-Step "Frontend: http://127.0.0.1:15174"
    Write-Step "Backend:  http://127.0.0.1:18081"
    if (-not $SkipAIClient) {
        Write-Step "AIClient2API: http://127.0.0.1:3000"
    }
}
