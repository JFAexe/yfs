# yfs - tiny go yaml file server

[![GitHub Release](https://img.shields.io/github/v/release/JFAexe/yfs?style=for-the-badge&color=%2300ADD8)](https://github.com/JFAexe/yfs/releases/latest)
[![License](https://img.shields.io/github/license/JFAexe/yfs?style=for-the-badge&color=%2300ADD8)](LICENSE)

> Just because you can, doesn't mean you should. Anyways...

```shell
echo '
---
- path: text/file.txt
  data: |-
    raw text file
- path: binary/file
  data: !!binary "eWZzCg=="
' | yfs -a 127.0.0.1 -p 1337
```

## Installation

> **DO NOT run any shell commands unless you understand them**

### Building

```shell
go install -trimpath -ldflags "-s -w" github.com/JFAexe/yfs/cmd/yfs@latest
```

### Prebuilt binaries

#### For Linux/Darwin

<details>
  <summary>Via shell</summary>

  ```shell
  (
    YFS_SYSYFS="darwin"
    YFS_ARCH="arm64"
    YFS_DOWNLOAD_PATH="$HOME/Downloads"
    YFS_INSTALL_PATH="$HOME/.local/bin/"

    YFS_URL=$(curl -sL https://api.github.com/repos/JFAexe/yfs/releases/latest | grep -o "https://[^\"]*${YFS_SYSYFS}_${YFS_ARCH}[^\"]*")
    YFS_ARCHIVE="$YFS_DOWNLOAD_PATH/${YFS_URL##*/}"

    curl -sL "$YFS_URL" -o "$YFS_ARCHIVE" && tar -xzf "$YFS_ARCHIVE" -C "$YFS_INSTALL_PATH" "yfs"
  )
  ```
</details>

#### For Windows

<details>
  <summary>Via powershell</summary>

  ```powershell
  $YFS_SYSYFS        = "windows"
  $YFS_ARCH          = "amd64"
  $YFS_DOWNLOAD_PATH = "$env:USERPROFILE\Downloads"
  $YFS_INSTALL_PATH  = "$env:LOCALAPPDATA\yfs"

  $RELEASE     = Invoke-RestMethod -Uri "https://api.github.com/repos/JFAexe/yfs/releases/latest"
  $YFS_URL     = $RELEASE.assets.browser_download_url | Where-Object { $_ -match "${YFS_SYSYFS}_${YFS_ARCH}" }
  $YFS_ARCHIVE = "$YFS_DOWNLOAD_PATH\$($YFS_URL.Split('/')[-1])"

  Invoke-WebRequest -Uri $YFS_URL -OutFile $YFS_ARCHIVE
  New-Item -ItemType Directory -Path $YFS_INSTALL_DIR -Force | Out-Null
  Expand-Archive -Path $YFS_ARCHIVE -DestinationPath $YFS_INSTALL_DIR -Force

  $ENV_PATH = [Environment]::GetEnvironmentVariable("Path", "User")

  if ($ENV_PATH -notlike "*$YFS_INSTALL_DIR*") {
    [Environment]::SetEnvironmentVariable("Path", "$ENV_PATH;$YFS_INSTALL_DIR", "User")
  }
  ```
</details>
