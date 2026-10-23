# Wails v3 release review

Reviewed 2026-10-23 against Wails v3.0.0-beta.34. The Go SDK, project-local
CLI and frontend runtime are pinned together. Go 1.26.2 is the project minimum;
optional obfuscation uses project-local Garble v0.16.0.

| Official guide | Project decision |
| --- | --- |
| [Building](https://v3.wails.io/guides/build/building/) | Use Wails v3 Taskfiles, generated bindings and production build tags. No Wails v2 commands or runtime imports. |
| [Customization](https://v3.wails.io/guides/build/customization/) | Retain maintained metadata, icons, native resources, per-platform tasks and common frontend/binding tasks. |
| [Cross-platform](https://v3.wails.io/guides/build/cross-platform/) | Windows EXE can cross-compile without CGO. Linux/macOS need native dependencies or the documented Docker toolchain. Native installation/update testing is required. |
| [Signing](https://v3.wails.io/guides/build/signing/) | Publisher signing and notarization remain optional and excluded from unsigned packaging. Update-payload authentication remains mandatory. |
| [Windows](https://v3.wails.io/guides/build/windows/) | Unsigned per-user NSIS installer and portable ZIP. Feed contains a raw application EXE, never an NSIS installer. |
| [macOS](https://v3.wails.io/guides/build/macos/) | Separate arm64/amd64 app bundles and DMGs. Retain Wails’ local ad-hoc bundle signature; no paid identity or notarization. Update ZIP has exactly one top-level app bundle. |
| [Private macOS APIs](https://v3.wails.io/guides/build/private-macos-apis/) | Public APIs remain the default. No private_mac_apis tag is needed for the current frameless UI. |
| [Linux](https://v3.wails.io/guides/build/linux/) | GTK4/WebKitGTK 6.0 only; AppImage, DEB, RPM and Arch package tasks. Native package dependencies include PolicyKit and the root-owned update helper. |
| [MSIX](https://v3.wails.io/guides/build/msix/) | Build target retained separately. Excluded from unsigned package:all and from claims of unsigned automatic-update support. |
| [Obfuscation](https://v3.wails.io/guides/build/obfuscation/) | Optional Wails v3 generated stable IDs, checked-in wails_obfuscated.gen.go, explicit JSON field tags, pinned compatible Garble. |
| [Updater](https://v3.wails.io/guides/updater/) | Use application.App.Updater, endpoint provider, SHA-512 and Ed25519ph, early updater.HandleHelperMode, and custom UI (WindowNone). |
| [Self-update tutorial](https://v3.wails.io/tutorials/04-self-update-a-wails-app/) | Use the v3 manifest/sign/verify commands, artifact-specific feeds and built-in executable/app-bundle replacement. Protect drafts and posting queues before restart. |

## Update behavior

Check ten seconds after startup, then every minute while open. The app owns the
polling loop so checks cannot replace an already staged release. It downloads
and authenticates automatically; the orange Restart to update action installs
once drafts are saved and all posting queues are idle.

Wails beta.34 handles Windows executables, macOS bundles and Linux AppImages
(including staging beside the outer AppImage rather than its mounted executable).
A separate AppImage swap implementation is unnecessary.

System packages are an explicit project extension: the built-in updater’s
staging directory beside the installed executable is root-owned, and swapping
that executable would bypass the package database. The package engine therefore
stages in the user cache, verifies through the same Wails provider and pinned
key, then requests PolicyKit authorization. The package-installed helper copies
the payload into root-private storage, verifies it again, checks identity,
version and architecture, and invokes apt/DNF/pacman. It never runs the GUI as
root. A successful transaction relaunches the new installed application after
the old process exits.

Package signing policies remain enabled. An unsigned RPM/Arch package may be
rejected by a system requiring package signatures; supported distro policies
must be tested before publication. Administrator approval is still required
where the native package manager needs it.

## What “without code signing” means

No Windows Authenticode certificate, Apple Developer ID, notarization, or Linux
publisher package signature is configured or invoked by package:all. macOS’s
local ad-hoc signature remains because it is part of the native Wails bundle
workflow and needs no paid certificate. Ed25519ph update signatures are also
retained; a checksum alone cannot authenticate a release.

MSIX is a platform constraint: normal sideloading needs a trusted package
signature, or distribution through the Store. Microsoft documents a special
unsigned-package route for Windows 11 testing, requiring a special identity and
administrator installation; that is not general public distribution support.
See [Microsoft’s unsigned MSIX guidance](https://learn.microsoft.com/en-us/windows/msix/package/unsigned-package).

## Release acceptance

`make package-all` builds the current desktop platform’s unsigned formats.
Cross-platform targets can be selected with the corresponding Wails task and
architecture variable. Homebrew uses the maintained cask template; publishing
a real tap is separate from producing a DMG.

The website, release feeds, native release CI and clean-machine old-to-new
update tests are still needed. Passing unit tests and cross-compilation does
not certify native macOS/Windows installation or Fedora/Arch transactions.
See [RELEASING.md](RELEASING.md) for feed names, payload construction and gates.

The README was checked against the source and these guides on 2026-10-23.
Wails Doctor reports no issues on the current Ubuntu 26.04 amd64 development
machine with CLI beta.34, Go 1.26.2, GTK4 and WebKitGTK 6.0. Its optional Docker
and legacy GTK3 entries are not requirements for the native desktop build.
