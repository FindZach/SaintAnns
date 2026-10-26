@echo off
echo Running database migrations...
go run main.go --migrate
if errorlevel 1 (
    echo Migrations failed.
    exit /b 1
)
echo Migrations completed.
