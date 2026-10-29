// Package safefs is file I/O for a program that writes into folders it does not own and reads files it did not
// write: opens that a symlinked last element cannot walk out of a folder, byte-capped reads, renames and removes that
// ride out Windows antivirus locks, writes and copies a crash leaves whole or absent, links and clones, and path
// lookup across Windows case. Subpackage diskcache keeps fetched answers on disk.
package safefs
