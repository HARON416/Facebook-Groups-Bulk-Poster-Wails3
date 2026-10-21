Release steps for Windows and Linux, in order (macOS deferred):

1. **Done: Search inside Postika.** Built-in Go search preserves prefix matching, typo tolerance, highlighted matches, and selections across searches. No separate search server is required.

2. **Done: Finalize release details.** Version `0.1.0`; publisher **Postika Technologies**; website **https://postika.softwrhq.com**; support **haronkibetrutoh@gmail.com**; distribution license **MIT**. Windows and Linux packages are unsigned.

3. **Prepare the app for installation.** Add single-instance handling, verify browser detection, and confirm saved groups/settings work from installed locations.

4. **Build and test packages.** Unsigned version `0.1.0` packages for Windows/Linux amd64 are built in `bin/releases/0.1.0`, with verified SHA-256 checksums. Installation and workflow testing remain.
   - Windows: installer with WebView2.
   - Linux: DEB and AppImage.
   
   Test installation, posting, search, response copying, and uninstalling on Windows and Linux.

5. **Deferred: Configure installer signing.** The current Windows and Linux release will be unsigned. Signed update manifests remain a separate future requirement.

6. **Publish downloads and add updates.** Put download buttons on your website, host verified files in GitHub Releases, and add a signed update manifest. Updates must wait until posting stops before restarting Postika. [Wails updater guide](https://v3.wails.io/guides/updater/)

**Next: prepare the app for installation.** Search and release details are complete. Add single-instance handling and verify browser detection and saved data from installed locations before testing beta installers.