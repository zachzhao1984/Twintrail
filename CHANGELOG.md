# Changelog

## 0.1.2

- No change to the app itself. This release re-runs the release build with
  a fix to the workflow that produced the 0.1.1 disk image, which was
  discarded before it could be attached to the release.

## 0.1.1

- In-app updates. Twintrail now checks for a new version in the background
  and offers to install it from a "Check for Updates…" item in the app
  menu, with the release notes of the new version. The update window is
  drawn in the app's own native UI, so no webview is involved.

## 0.1.0

The first release of Twintrail, a native macOS dual-pane file manager.

- Directories first, name ascending or descending, hidden-file toggle, and a
  name substring filter.
- Copy, move, rename, create nested directories, and move to the macOS Trash,
  with confirmation dialogs and error display. Copying preserves symlinks,
  refuses to overwrite an existing target, and refuses to copy or move a
  directory into itself. Cross-volume moves report an error rather than
  falling back to a copy and a delete.
- File operations and directory reads run in the background, and a stale
  directory read is discarded rather than shown.
- Open with the system default application, Quick Look preview, reveal in
  Finder, and an external Terminal.
- A native context menu on every row, acting on the row it was opened on
  rather than on whichever row the cursor was on. Its Copy Path item writes
  the absolute paths of the marked entries to the pasteboard, one per line,
  each quoted so that pasting into a terminal is safe.
- Opening the context menu moves the keyboard highlight to that row as the
  menu appears, instead of leaving it on the old row until the menu closes.
- Nine font settings for the action bar, breadcrumbs, virtual tabs, file
  list, status bar, table header, tabs, preferences and terminal, applied as
  they are changed and saved to the window state.
- Open With, listing the applications of the selected files through
  NSWorkspace, with icons rendered in the background and cached.
- An embedded terminal per tab, with its own zsh login shell and scrollback,
  toggled from View, whose working directory follows navigation without
  injecting commands into its input.
- In-app updates, checked in the background and on demand from the app menu.