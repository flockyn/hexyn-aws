$CORE_REPO = "flockyn/hexyn-aws"
$installDir = "$HOME\.hexyn-aws\bin"
$destPath = "$installDir\hexyn-aws.exe"

Write-Host "🚀 Starting Hexyn AWS installation..." -ForegroundColor Cyan

# 1. Resolve the matching asset from the latest public release.
#    No GitHub token is required (the repo is public); GitHub only needs a User-Agent header.
$headers = @{ "User-Agent" = "hexyn-aws-installer" }
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$CORE_REPO/releases/latest" -Headers $headers
$asset = $release.assets | Where-Object { $_.name -like "hexyn-aws_windows_amd64.zip" }

if ($null -eq $asset) {
    Write-Host "❌ Could not find the Windows binary in the latest release." -ForegroundColor Red
    exit 1
}

# 2. Download and Extract
if (!(Test-Path $installDir)) { New-Item -ItemType Directory -Path $installDir | Out-Null }
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile "$installDir\hexyn-aws.zip"
Expand-Archive -Path "$installDir\hexyn-aws.zip" -DestinationPath $installDir -Force
Remove-Item "$installDir\hexyn-aws.zip"

# 3. PATH Update
$path = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($path -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$path;$installDir", "User")
    Write-Host "⚠️ PATH updated. Please restart your terminal." -ForegroundColor Yellow
}

Write-Host "✅ Installed! Type 'hexyn-aws' to start." -ForegroundColor Green
