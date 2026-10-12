@echo off
cd /d "%~dp0"
echo Starting ZhiShu...
docker compose up -d
if %errorlevel% neq 0 (
    echo [Error] Failed to start Docker containers.
    pause
    exit /b 1
)
echo.
echo ZhiShu is running at http://localhost
echo Following logs (Press Ctrl+C to exit logs)...
docker compose logs -f
