@echo off
cd /d "%~dp0"

rem ---- locate Python ----
set "PY="
where py >nul 2>nul && set "PY=py -3"
if not defined PY where python >nul 2>nul && set "PY=python"
if not defined PY (
    echo [ERROR] Python not found. Please install Python 3 and add to PATH.
    pause
    exit /b 1
)

echo [INFO] Interpreter: %PY%
echo [INFO] Checking dependencies...
%PY% -c "import importlib.util as u; mods=['keyboard','win32com','pystray','PIL','comtypes']; miss=[m for m in mods if u.find_spec(m) is None]; print('  MISSING: ' + ', '.join(miss)) if miss else print('  All OK')"

echo [INFO] Starting toggle_desktop_icons.py  (Ctrl+C to quit)
echo ===============================================================
%PY% toggle_desktop_icons.py
echo ===============================================================
echo [INFO] Exited with code: %errorlevel%
pause
