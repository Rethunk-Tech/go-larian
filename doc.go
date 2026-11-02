// Package larian reads Larian Studios' mod formats as Baldur's Gate 3 uses them:
// LSPK .pak archives, a mod's meta.lsx, and the modsettings.lsx load order.
//
// Sources, all read from their repositories rather than recalled:
//
//   - Norbyte/lslib, LSLib/LS/PackageFormat.cs (header structs LSPKHeader15 and
//     LSPKHeader16, file entries FileEntry15 at 296 bytes for versions 15 and 16 and
//     FileEntry18 at 272 bytes for version 18, PackageFlags), PackageReader.cs
//     (signature "LSPK" at 0, version at 4, header at 4; LZ4-block-compressed file
//     list at FileListOffset holding int32 count and int32 compressed size),
//     LSLib/LS/Enums/Compression.cs (low nibble: 0 none, 1 zlib, 2 LZ4, 3 zstd;
//     high bits 0x10, 0x20, 0x40 are the level and are ignored when reading) and
//     PackageCommon.cs (deletion offset 0xbeefdeadbeef).
//   - LaughingLeader/BG3ModManager, src/Core/DivinityApp.cs (modsettings.lsx
//     template: <version major=4 minor=8 revision=0 build=100>, a Mods node of
//     ModuleShortDesc nodes, no ModOrder; base module UUIDs) and
//     src/Core/Models/DivinityModVersion2.cs (Version64 split 9/8/16/31 bits).
//
// Supported package versions are 15, 16 and 18; compression methods are none,
// zlib and LZ4 (block). Multi-part archives (ArchivePart != 0) and zstd entries
// are reported as [ErrUnsupported] when read, and solid packages when opened.
// Every size, offset, count and path in a package is untrusted and bounds-checked.
package larian
