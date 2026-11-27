@echo off
REM DIOFOS Zero-Install Windows Launcher
REM Runs the standalone 64-bit Windows binary without needing Go or Make.

set DIR=%~dp0
set BINARY=%DIR%bin\dist\diofos-windows-amd64.exe

if not exist "%BINARY%" (
    if exist "%DIR%bin\diofos.exe" (
        set BINARY=%DIR%bin\diofos.exe
    ) else (
        echo Error: Windows pre-compiled binary not found at %BINARY%
        pause
        exit /b 1
    )
)

"%BINARY%" %*
