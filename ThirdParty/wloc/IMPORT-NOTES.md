# WLOC vendored copy

Imported from https://github.com/Yu9191/wloc
Upstream author/project attribution: Yu9191 Rewrite.
Upstream license: GNU Affero General Public License v3.0 (see LICENSE).
Imported into this repository on 2026-11-17 under ThirdParty/wloc/.

This directory is a vendored snapshot. Keep upstream notices and license intact.
No iOS IPA is included in this import. WLOC is primarily proxy rewrite scripts and a
Cloudflare Worker-based location picker, not a standalone iOS application.

Compatibility note: importing these files does not establish that MITM interception
of Apple's gs-loc endpoints works on a given iOS version. Test the proxy certificate,
hostname interception, and actual locationd behavior on-device before relying on it.
