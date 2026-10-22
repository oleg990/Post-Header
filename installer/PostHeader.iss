; Установщик Post Header (Inno Setup 6.2). Ставится без прав администратора
; в профиль пользователя, создаёт ярлыки, на 64-битной Windows ставит
; 64-битную сборку. Собирается в CI: iscc /DAppVersion=0.1.0 PostHeader.iss

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#define Dist "..\dist"
; Arch=win64 или win32 — облегчённый установщик для автообновления:
; в нём только одна сборка, поэтому он скачивается вдвое быстрее.
#ifndef Arch
  #define Arch ""
#endif

[Setup]
AppId={{6F1D2B7A-4C3E-4B8F-9A5D-2E7C1F0A9B31}
AppName=Post Header
AppVersion={#AppVersion}
AppVerName=Post Header {#AppVersion}
AppPublisher=Олег
AppPublisherURL=https://github.com/oleg990/Post-Header
DefaultDirName={autopf}\Post Header
DefaultGroupName=Post Header
DisableProgramGroupPage=yes
DisableDirPage=yes
DisableReadyPage=yes
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64
MinVersion=6.1sp1
OutputDir={#Dist}
#if Arch == ""
OutputBaseFilename=PostHeader-Setup-v{#AppVersion}
#else
OutputBaseFilename=PostHeader-Update-{#Arch}
#endif
#if Arch == "win64"
ArchitecturesAllowed=x64
#endif
SetupIconFile=icon.ico
UninstallDisplayIcon={app}\PostHeader.exe
UninstallDisplayName=Post Header
WizardStyle=modern
Compression=lzma2/max
SolidCompression=yes
CloseApplications=force
RestartApplications=no

[Languages]
Name: "ru"; MessagesFile: "compiler:Languages\Russian.isl"

[Tasks]
Name: "desktopicon"; Description: "Ярлык на рабочем столе"; GroupDescription: "Ярлыки:"

[Files]
#if Arch == ""
Source: "{#Dist}\PostHeader-win64.exe"; DestDir: "{app}"; DestName: "PostHeader.exe"; Check: Is64BitInstallMode; Flags: ignoreversion
Source: "{#Dist}\PostHeader-win32.exe"; DestDir: "{app}"; DestName: "PostHeader.exe"; Check: not Is64BitInstallMode; Flags: ignoreversion
#else
Source: "{#Dist}\PostHeader-{#Arch}.exe"; DestDir: "{app}"; DestName: "PostHeader.exe"; Flags: ignoreversion
#endif

[Icons]
Name: "{group}\Post Header"; Filename: "{app}\PostHeader.exe"
Name: "{group}\Удалить Post Header"; Filename: "{uninstallexe}"
Name: "{autodesktop}\Post Header"; Filename: "{app}\PostHeader.exe"; Tasks: desktopicon

[Run]
; Без skipifsilent: после тихого автообновления программа откроется снова.
Filename: "{app}\PostHeader.exe"; Description: "Запустить Post Header"; Flags: nowait postinstall
