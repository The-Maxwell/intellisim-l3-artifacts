@echo off
echo Starting IntelliSim Development Environment...
echo.

REM Check if MongoDB is running
echo Checking MongoDB...
sc query MongoDB | find "RUNNING" >nul
if errorlevel 1 (
    echo MongoDB is not running. Please start MongoDB first.
    echo Run: net start MongoDB
    pause
    exit /b 1
)
echo MongoDB is running.
echo.

REM Start backend in a new window
echo Starting Backend Server...
start "IntelliSim Backend" cmd /k "go run ./cmd/intelli-sim"

REM Wait a bit for backend to start
timeout /t 5 /nobreak >nul

REM Start frontend in a new window
echo Starting Frontend Server...
start "IntelliSim Frontend" cmd /k "cd ui && npm run dev"

echo.
echo ========================================
echo IntelliSim is starting...
echo ========================================
echo Backend: http://localhost:8080
echo Frontend: http://localhost:3000
echo ========================================
echo.
echo Press any key to stop all servers...
pause >nul

REM Kill processes when user presses a key
echo Stopping servers...
taskkill /FI "WINDOWTITLE eq IntelliSim Backend*" /F >nul 2>&1
taskkill /FI "WINDOWTITLE eq IntelliSim Frontend*" /F >nul 2>&1
echo Servers stopped.
