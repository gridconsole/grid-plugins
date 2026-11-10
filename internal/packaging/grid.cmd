@echo off
rem Generated as bin\grid.cmd by make generate; edit internal/packaging/grid.cmd.
rem Runs the Windows binary shipped beside this file. It needs no Go and no network.
setlocal
set "grid_exe=%~dp0grid-windows-amd64.exe"
if not exist "%grid_exe%" (
  echo grid: missing executable for windows-amd64; install a complete release 1>&2
  exit /b 1
)
"%grid_exe%" %*
exit /b %ERRORLEVEL%
