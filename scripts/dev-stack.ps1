param(
    [ValidateSet("start", "restart", "stop", "status", "run")]
    [string]$Action = "restart",

    [ValidateSet("backend", "frontend")]
    [string]$Component,

    [string]$AIClientPath = "",

    [switch]$SkipAIClient,

    [string]$NewAPIPath = "",

    [switch]$IncludeNewAPI,

    [int]$NewAPIPort = 13200,

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
if ([string]::IsNullOrWhiteSpace($NewAPIPath)) {
    $NewAPIPath = Join-Path (Split-Path -Parent $RepoRoot) "new-api"
}
$NewAPIComposeFile = Join-Path $StateDir "new-api.compose.yml"
$NewAPIComposeProject = "sub2api-new-api-dev"

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
        if ($entry.Kind -eq "compose") {
            Stop-ComposeService -Service $entry
        }
        elseif ($entry.PID) {
            Write-Step "Stopping $($entry.Name) pid=$($entry.PID)"
            Stop-ProcessTree -ProcessId ([int]$entry.PID)
        }
    }
    Save-State -Processes @()
}

function Stop-PortProcesses {
    param([array]$Services)

    foreach ($service in $Services) {
        if ($service.Kind -eq "compose") {
            continue
        }
        foreach ($port in $service.Ports) {
            $ids = Get-PortProcessIds -Port $port
            foreach ($id in $ids) {
                Write-Step "Stopping process on port $port for $($service.Name), pid=$id"
                Stop-ProcessTree -ProcessId ([int]$id)
            }
        }
    }
}

function Stop-ComposeServices {
    param([array]$Services)

    foreach ($service in $Services) {
        if ($service.Kind -eq "compose") {
            Stop-ComposeService -Service $service
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
        Kind = "process"
        Name = $Name
        PID = $process.Id
        Port = $Port
        Ports = $Ports
        WorkingDirectory = $WorkingDirectory
        Command = $Command
        Stdout = $stdout
        Stderr = $stderr
        StartedAt = (Get-Date).ToString("s")
    }
}

function Invoke-ServiceForeground {
    param([object]$Service)

    if ($Service.Kind -eq "compose") {
        throw "Foreground mode does not support compose service $($Service.Name)."
    }
    if (-not (Test-Path $Service.WorkingDirectory)) {
        throw "$($Service.Name) working directory does not exist: $($Service.WorkingDirectory)"
    }

    foreach ($port in $Service.Ports) {
        if (Test-PortOpen -HostName "127.0.0.1" -Port ([int]$port)) {
            throw "$($Service.Name) cannot start because 127.0.0.1:$port is already listening."
        }
    }

    Write-Step "Running $($Service.Name) in the foreground on port $($Service.Port)"
    Push-Location -LiteralPath $Service.WorkingDirectory
    try {
        & ([scriptblock]::Create($Service.Command))
        if ($LASTEXITCODE -is [int] -and $LASTEXITCODE -ne 0) {
            throw "$($Service.Name) exited with code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
}

function Invoke-Compose {
    param(
        [object]$Service,
        [string[]]$Arguments,
        [string]$LogFile
    )

    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw "Docker CLI is required to manage $($Service.Name)."
    }

    $output = & docker compose -p $Service.ComposeProject -f $Service.ComposeFile @Arguments 2>&1
    $exitCode = $LASTEXITCODE
    if ($output) {
        $output | Tee-Object -FilePath $LogFile -Append | Out-Host
    }
    if ($exitCode -ne 0) {
        throw "docker compose $($Arguments -join ' ') failed for $($Service.Name) with exit code $exitCode. See $LogFile."
    }
}

function New-NewAPIComposeFile {
    param(
        [string]$WorkingDirectory,
        [string]$ComposeFile,
        [int]$Port
    )

    if (-not (Test-Path $WorkingDirectory)) {
        throw "new-api working directory does not exist: $WorkingDirectory"
    }

    $contextPath = (Resolve-Path -LiteralPath $WorkingDirectory).Path.Replace("\", "/")
    $composeContent = @"
services:
  new-api:
    build:
      context: "$contextPath"
      dockerfile: Dockerfile.dev
    image: new-api-dev:local
    container_name: sub2api-new-api-dev
    restart: unless-stopped
    ports:
      - "127.0.0.1:${Port}:3000"
    volumes:
      - new_api_dev_data:/data
    environment:
      - SQL_DSN=postgresql://root:123456@postgres:5432/new-api
      - REDIS_CONN_STRING=redis://redis
      - TZ=Asia/Shanghai
      - BATCH_UPDATE_ENABLED=true
    depends_on:
      redis:
        condition: service_started
      postgres:
        condition: service_healthy
    networks:
      - new_api_dev_network

  redis:
    image: redis:7-alpine
    container_name: sub2api-new-api-dev-redis
    restart: unless-stopped
    networks:
      - new_api_dev_network

  postgres:
    image: postgres:15-alpine
    container_name: sub2api-new-api-dev-pg
    restart: unless-stopped
    environment:
      POSTGRES_USER: root
      POSTGRES_PASSWORD: 123456
      POSTGRES_DB: new-api
    volumes:
      - new_api_dev_pg_data:/var/lib/postgresql/data
    networks:
      - new_api_dev_network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U root -d new-api"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  new_api_dev_data:
  new_api_dev_pg_data:

networks:
  new_api_dev_network:
    driver: bridge
"@

    Set-Content -Path $ComposeFile -Value $composeContent -Encoding UTF8
}

function Start-ComposeService {
    param([object]$Service)

    $stdout = Join-Path $LogDir "$($Service.Name).compose.log"
    Write-Step "Starting $($Service.Name) on port $($Service.Port)"

    if ($Service.Name -eq "new-api") {
        New-NewAPIComposeFile `
            -WorkingDirectory $Service.WorkingDirectory `
            -ComposeFile $Service.ComposeFile `
            -Port $Service.Port
    }

    Invoke-Compose `
        -Service $Service `
        -Arguments @("up", "-d", "--build") `
        -LogFile $stdout

    return [pscustomobject]@{
        Kind = "compose"
        Name = $Service.Name
        PID = $null
        Port = $Service.Port
        Ports = $Service.Ports
        WorkingDirectory = $Service.WorkingDirectory
        ComposeFile = $Service.ComposeFile
        ComposeProject = $Service.ComposeProject
        Stdout = $stdout
        Stderr = $null
        StartedAt = (Get-Date).ToString("s")
    }
}

function Stop-ComposeService {
    param([object]$Service)

    if (-not $Service.ComposeFile -or -not (Test-Path $Service.ComposeFile)) {
        Write-Warning "Cannot stop $($Service.Name): compose file is missing at $($Service.ComposeFile)."
        return
    }

    $logFile = if ($Service.Stdout) { $Service.Stdout } else { Join-Path $LogDir "$($Service.Name).compose.log" }
    Write-Step "Stopping $($Service.Name) compose project $($Service.ComposeProject)"
    Invoke-Compose `
        -Service $Service `
        -Arguments @("down") `
        -LogFile $logFile
}

function Show-Status {
    $state = Read-State
    if ($state.Count -eq 0) {
        Write-Step "No managed dev-stack processes are recorded."
    }

    foreach ($entry in $state) {
        if ($entry.Kind -eq "compose") {
            $stateText = if (Test-PortOpen -HostName "127.0.0.1" -Port ([int]$entry.Port)) { "running" } else { "stopped" }
            $pidText = "-"
        }
        else {
            $process = Get-Process -Id ([int]$entry.PID) -ErrorAction SilentlyContinue
            $stateText = if ($null -eq $process) { "stopped" } else { "running" }
            $pidText = $entry.PID
        }
        $ports = @($entry.Ports)
        if ($ports.Count -eq 0 -and $entry.Port) {
            $ports = @($entry.Port)
        }
        $portStates = foreach ($port in $ports) {
            $portText = if (Test-PortOpen -HostName "127.0.0.1" -Port ([int]$port)) { "listening" } else { "not listening" }
            "${port}:${portText}"
        }
        Write-Host ("{0,-12} pid={1,-8} ports={2,-28} {3}" -f $entry.Name, $pidText, ($portStates -join ", "), $stateText)
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
        Kind = "process"
        Name = "aiclient2api"
        WorkingDirectory = $AIClientPath
        Command = "pnpm start"
        Port = 3000
        Ports = @(3000, 3100)
    }
}

if ($IncludeNewAPI) {
    $services += [pscustomobject]@{
        Kind = "compose"
        Name = "new-api"
        WorkingDirectory = $NewAPIPath
        ComposeFile = $NewAPIComposeFile
        ComposeProject = $NewAPIComposeProject
        Port = $NewAPIPort
        Ports = @($NewAPIPort)
    }
}

if ($Action -eq "run") {
    if ([string]::IsNullOrWhiteSpace($Component)) {
        throw "-Component backend or -Component frontend is required with the run action."
    }

    $selectedService = $services | Where-Object { $_.Name -eq $Component } | Select-Object -First 1
    if ($null -eq $selectedService) {
        throw "Unknown foreground component: $Component"
    }

    foreach ($dependency in @(
        [pscustomobject]@{ Name = "PostgreSQL"; Port = 5432 },
        [pscustomobject]@{ Name = "Redis"; Port = 6379 }
    )) {
        if (-not (Test-PortOpen -HostName "127.0.0.1" -Port $dependency.Port)) {
            Write-Warning "$($dependency.Name) is not reachable on 127.0.0.1:$($dependency.Port). Start Docker Desktop/dev containers first if this service is required."
        }
    }

    Invoke-ServiceForeground -Service $selectedService
    exit 0
}

switch ($Action) {
    "status" {
        Show-Status
        break
    }
    "stop" {
        Stop-ManagedProcesses
        Stop-ComposeServices -Services $services
        Stop-PortProcesses -Services $services
        Show-Status
        break
    }
    "restart" {
        Stop-ManagedProcesses
        Stop-ComposeServices -Services $services
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
        if ($service.Kind -eq "compose") {
            $started += Start-ComposeService -Service $service
        }
        else {
            $started += Start-ServiceProcess `
                -Name $service.Name `
                -WorkingDirectory $service.WorkingDirectory `
                -Command $service.Command `
                -Port $service.Port `
                -Ports $service.Ports
        }
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
    if ($IncludeNewAPI) {
        Write-Step "new-api: http://127.0.0.1:$NewAPIPort"
    }
}
