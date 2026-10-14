; Luci 0.3 installed these exact Go licence paths. Later generated NSIS
; uninstallers know only their current resources, so remove these obsolete
; package files explicitly. Never recurse or touch saved workspace/user data.
!include "LogicLib.nsh"

; Tauri CLI 2.12.1 includes this file before expanding the standard MUI pages.
; Keep the upstream template and its licence/upgrade/shortcut behaviour.
!define MUI_WELCOMEPAGE_TITLE "Welcome to Luci"
!define MUI_WELCOMEPAGE_TEXT "Discover, Design, Develop and Deploy in one desktop workspace.$\r$\n$\r$\nThis installs Luci for your Windows account. On first launch, choose Local or connect your own supported cloud.$\r$\n$\r$\nFor local coding sessions, Docker Desktop must be installed and running. Luci Cloud is not yet available.$\r$\n$\r$\nClick Next to continue."
!define MUI_FINISHPAGE_TITLE "Luci is installed"
!define MUI_FINISHPAGE_TEXT "Open Luci to set up your workspace and choose where coding sessions run.$\r$\n$\r$\nProject review and runtime setup continue inside the app."

!macro LUCI_CHECK_UNLINKED PATH
  System::Call 'kernel32::GetFileAttributesW(w "${PATH}") i .r0'
  ${If} $0 != -1
    IntOp $0 $0 & 0x400 ; FILE_ATTRIBUTE_REPARSE_POINT includes junctions.
    ${If} $0 != 0
      StrCpy $1 1
    ${EndIf}
  ${EndIf}
!macroend

!macro LUCI_REMOVE_03_LICENSE DIRECTORY
  StrCpy $1 0
  !insertmacro LUCI_CHECK_UNLINKED "$INSTDIR"
  !insertmacro LUCI_CHECK_UNLINKED "$INSTDIR\licenses"
  !insertmacro LUCI_CHECK_UNLINKED "$INSTDIR\licenses\go"
  !insertmacro LUCI_CHECK_UNLINKED "$INSTDIR\licenses\go\${DIRECTORY}"
  !insertmacro LUCI_CHECK_UNLINKED "$INSTDIR\licenses\go\${DIRECTORY}\LICENSE"
  ${If} $1 == 0
    Delete "$INSTDIR\licenses\go\${DIRECTORY}\LICENSE"
    ; Nonrecursive: any unknown file keeps its directory intact.
    RMDir "$INSTDIR\licenses\go\${DIRECTORY}"
  ${EndIf}
!macroend

!macro NSIS_HOOK_PREUNINSTALL
  Push $0
  Push $1
  !insertmacro LUCI_REMOVE_03_LICENSE "golang.org_x_net-v0.26.0"
  !insertmacro LUCI_REMOVE_03_LICENSE "golang.org_x_sys-v0.24.0"
  !insertmacro LUCI_REMOVE_03_LICENSE "golang.org_x_term-v0.21.0"
  !insertmacro LUCI_REMOVE_03_LICENSE "golang.org_x_text-v0.16.0"
  Pop $1
  Pop $0
  ; Fresh installs have none of these files. Missing paths and preserved
  ; unknown files must not contaminate the generated uninstaller's error flag.
  ClearErrors
!macroend
