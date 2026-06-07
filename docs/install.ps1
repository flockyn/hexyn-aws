$CORE_REPO = "flockyn/hexyn-aws"
$installDir = "$HOME\.hexyn-aws\bin"
$destPath = "$installDir\hexyn-aws.exe"

Write-Host "🚀 Starting Hexyn AWS installation..." -ForegroundColor Cyan

# 1. Get Token
$tokenStr = $env:GITHUB_TOKEN
if ([string]::IsNullOrWhiteSpace($tokenStr)) {
    $token = Read-Host "🔑 Private tool: Enter your GitHub Token" -AsSecureString
    $tokenStr = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto([System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($token))
}

# 2. Get Asset Info
$headers = @{ "Authorization" = "Bearer $tokenStr" }
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$CORE_REPO/releases/latest" -Headers $headers
$asset = $release.assets | Where-Object { $_.name -like "hexyn-aws_windows_amd64.zip" }

if ($null -eq $asset) {
    Write-Host "❌ Could not find binary in latest release. Check your token and repo name." -ForegroundColor Red
    exit 1
}

# 3. Download and Extract
if (!(Test-Path $installDir)) { New-Item -ItemType Directory -Path $installDir }
Invoke-WebRequest -Uri $asset.url -Headers @{ "Authorization" = "Bearer $tokenStr"; "Accept" = "application/octet-stream" } -OutFile "$installDir\hexyn-aws.zip"

Expand-Archive -Path "$installDir\hexyn-aws.zip" -DestinationPath $installDir -Force
Remove-Item "$installDir\hexyn-aws.zip"

# 4. PATH Update
$path = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($path -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$path;$installDir", "User")
    Write-Host "⚠️ PATH updated. Please restart your terminal." -ForegroundColor Yellow
}

Write-Host "✅ Installed! Type 'hexyn-aws' to start." -ForegroundColor Green
