go build -ldflags "-s -w" -o goide.exe .
if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful: goide.exe" -ForegroundColor Green
} else {
    Write-Host "Build failed" -ForegroundColor Red
    exit $LASTEXITCODE
}
