# -*- mode: python ; coding: utf-8 -*-


a = Analysis(
    ['toggle_desktop_icons.py'],
    pathex=[],
    binaries=[],
    datas=[('1.png', '.'), ('2.png', '.')],
    hiddenimports=['pystray._win32', 'win32com', 'pythoncom', 'win32api', 'win32gui', 'win32con',
                   'comtypes', 'comtypes.gen', 'comtypes.gen.stdole',
                   'comtypes.gen.Accessibility',
                   'comtypes.gen._00020430_0000_0000_C000_000000000046_0_2_0',
                   'comtypes.gen._1EA4DBF0_3C3B_11CF_810C_00AA00389B71_0_1_1',
                   'pkg_resources'],
    hookspath=[],
    runtime_hooks=[],
    excludes=[],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.datas,
    [],
    name='DesktopToggle',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=False,
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)
