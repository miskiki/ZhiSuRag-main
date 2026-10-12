@echo off
cd /d "%~dp0"
echo Stopping ZhiShu containers...
docker compose stop
echo.
echo ZhiShu has been stopped.
pause
