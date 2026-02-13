@echo off
setlocal

set "PS1_PATH="
set "PS1_CANDIDATE=%~dp0..\..\integration\windows\run-release-integration.ps1"
if exist "%PS1_CANDIDATE%" (
  set "PS1_PATH=%PS1_CANDIDATE%"
)
set "PS1_CANDIDATE=%~dp0..\..\..\integration\windows\run-release-integration.ps1"
if exist "%PS1_CANDIDATE%" (
  set "PS1_PATH=%PS1_CANDIDATE%"
)

set "GORILLA_EXE=%ProgramData%\gorilla\bin\gorilla.exe"
set "WORK_ROOT=%TEMP%\gorilla-release-integration"
set "EXITCODE=0"

if not "%~1"=="" (
  set "GORILLA_EXE=%~1"
)

if not "%~2"=="" (
  set "WORK_ROOT=%~2"
)

if "%PS1_PATH%"=="" (
  echo Missing integration script. Checked:
  echo   %~dp0..\..\integration\windows\run-release-integration.ps1
  echo   %~dp0..\..\..\integration\windows\run-release-integration.ps1
  set "EXITCODE=1"
  goto end
)

echo Running Windows release integration test
echo Gorilla exe: %GORILLA_EXE%
echo Work root : %WORK_ROOT%
echo.

powershell -NoProfile -NoLogo -NonInteractive -ExecutionPolicy Bypass -File "%PS1_PATH%" -GorillaExePath "%GORILLA_EXE%" -WorkRoot "%WORK_ROOT%"
set "EXITCODE=%errorlevel%"

:end
echo.
pause
exit /b %EXITCODE%
