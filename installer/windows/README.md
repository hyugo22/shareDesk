# Installeur Windows de l'agent ShareDesk

MSI construit avec [WiX Toolset v5](https://wixtoolset.org/) à partir de
[`ShareDeskAgent.wxs`](ShareDeskAgent.wxs). Compilé automatiquement par la CI
(`.github/workflows/ci.yml`, job `windows-agent-installer`) sur un runner
Windows — WiX ne garantit pas de fonctionner correctement en dehors de
Windows, il ne doit donc pas être compilé ailleurs.

## Construction manuelle (sur Windows)

```powershell
# Build des binaires agent + icône de zone de notification
cd agent
go build -o ..\installer\windows\sharedesk-agent.exe .\cmd\agent
go build -o ..\installer\windows\sharedesk-agent-tray.exe .\cmd\tray

# Installation de l'outil WiX (une fois)
dotnet tool install --global wix --version 5.0.2

cd ..\installer\windows
wix build ShareDeskAgent.wxs `
  -d AgentVersion=0.1.0 `
  -d AgentBinaryPath=sharedesk-agent.exe `
  -d TrayBinaryPath=sharedesk-agent-tray.exe `
  -o ShareDeskAgent.msi
```

## Déploiement silencieux (GPO ou script)

```powershell
msiexec /i ShareDeskAgent.msi /qn `
  SERVER_URL="https://sharedesk.example.com:8080" `
  MTLS_HOST="sharedesk.example.com:8443" `
  ENROLLMENT_TOKEN="<token généré depuis Administration > Agents > Ajouter une machine>"
```

L'installeur copie les exécutables dans `Program Files\ShareDesk Agent\`, puis
délègue à `sharedesk-agent.exe install` (voir
[`agent/cmd/agent/main.go`](../../agent/cmd/agent/main.go)) l'enrôlement
auprès du serveur, l'enregistrement du service Windows
(`github.com/kardianos/service`), et l'inscription de l'icône de zone de
notification (`sharedesk-agent-tray.exe`, voir
[`agent/cmd/tray`](../../agent/cmd/tray)) au démarrage de session (clé
Run HKLM — visible à la prochaine connexion, un service tournant en
Session 0 ne pouvant pas afficher d'UI sur le bureau de l'utilisateur). Le
token n'est utilisé qu'une fois, à cet instant précis — il n'est jamais
écrit sur disque par le MSI ni conservé par l'agent au-delà de l'échange
initial.

Désinstallation : `msiexec /x ShareDeskAgent.msi /qn` arrête et supprime le
service avant de retirer les fichiers (l'identité mTLS de l'agent, elle,
n'est pas supprimée — voir `%ProgramData%\ShareDeskAgent\data`).

## Limitations connues

- Le MSI n'est pas signé (aucun certificat de signature de code n'est
  configuré dans ce projet) : SmartScreen/Defender peuvent avertir à
  l'installation. La signature est un prérequis pour un déploiement en
  production, voir docs/SECURITY.md.
- Non testé sur une machine Windows réelle à ce stade (environnement de
  développement Linux uniquement) — le job CI `windows-agent-installer` sur
  runner `windows-latest` est le premier passage sur un environnement
  Windows authentique.
