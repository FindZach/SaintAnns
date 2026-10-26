Write-Host "Running database migrations..." -ForegroundColor Yellow
go run main.go --migrate
if ($LASTEXITCODE -ne 0) {
    Write-Host "Migrations failed." -ForegroundColor Red
    exit 1
}
Write-Host "Migrations completed." -ForegroundColor Green
