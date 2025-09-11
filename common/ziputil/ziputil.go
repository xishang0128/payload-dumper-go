package ziputil

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/xishang0128/payload-dumper-go/common/file"
)

const (
	zipEOCDMagic      = 0x06054b50
	zip64EOCDMagic    = 0x06064b50
	zip64LocatorMagic = 0x07064b50
	zipCDFHMagic      = 0x02014b50
	zipFHMagic        = 0x04034b50
	zipEOCDSize       = 22
	zip64EOCDSize     = 56
	zip64LocatorSize  = 20
	zipCDFHSize       = 46
	zipFHSize         = 30
	zipMaxComment     = 65535
	zipStored         = 0
)

type zipEOCD struct {
	DiskNumber      uint16
	CDDiskNumber    uint16
	CDRecordsOnDisk uint16
	CDRecords       uint16
	CDSize          uint32
	CDOffset        uint32
	CommentLength   uint16
}

type zip64EOCD struct {
	RecordSize      uint64
	VersionMadeBy   uint16
	VersionNeeded   uint16
	DiskNumber      uint32
	CDDiskNumber    uint32
	CDRecordsOnDisk uint64
	CDRecords       uint64
	CDSize          uint64
	CDOffset        uint64
}

type zip64Locator struct {
	DiskNumber uint32
	EOCDOffset uint64
	TotalDisks uint32
}

type zipCDFH struct {
	VersionMadeBy     uint16
	VersionNeeded     uint16
	Flags             uint16
	CompressionMethod uint16
	ModTime           uint16
	ModDate           uint16
	CRC32             uint32
	CompressedSize    uint32
	UncompressedSize  uint32
	FileNameLength    uint16
	ExtraFieldLength  uint16
	FileCommentLength uint16
	DiskNumberStart   uint16
	InternalFileAttrs uint16
	ExternalFileAttrs uint32
	LocalHeaderOffset uint32
}

type zipFH struct {
	VersionNeeded     uint16
	Flags             uint16
	CompressionMethod uint16
	ModTime           uint16
	ModDate           uint16
	CRC32             uint32
	CompressedSize    uint32
	UncompressedSize  uint32
	FileNameLength    uint16
	ExtraFieldLength  uint16
}

func GetStoredEntryOffset(reader file.Reader, name string) (int64, int64, error) {
	size := reader.Size()

	if size < zipEOCDSize {
		return 0, 0, fmt.Errorf("not enough length to contain EOCD")
	}

	// Find EOCD
	var eocdOffset int64 = -1

	// Try reading from the end first
	data, err := reader.Read(size-zipEOCDSize, zipEOCDSize)
	if err != nil {
		return 0, 0, err
	}

	if len(data) == zipEOCDSize && binary.LittleEndian.Uint32(data[0:4]) == zipEOCDMagic &&
		binary.LittleEndian.Uint16(data[20:22]) == 0 {
		eocdOffset = size - zipEOCDSize
	} else {
		// Search for EOCD with comment
		trySize := int64(zipMaxComment + zipEOCDSize)
		start := size - trySize
		if start < 0 {
			start = 0
			trySize = size
		}

		searchData, err := reader.Read(start, int(trySize))
		if err != nil {
			return 0, 0, err
		}

		for length := 1; length < int(trySize)-zipEOCDSize+1; length++ {
			if length+2 > len(searchData) {
				break
			}
			commentLen := binary.LittleEndian.Uint16(searchData[len(searchData)-length-2 : len(searchData)-length])
			if int(commentLen) == length {
				pos := len(searchData) - length - zipEOCDSize
				if pos >= 0 && pos+4 <= len(searchData) {
					if binary.LittleEndian.Uint32(searchData[pos:pos+4]) == zipEOCDMagic {
						data = searchData[pos : pos+zipEOCDSize]
						eocdOffset = size - int64(length) - zipEOCDSize
						break
					}
				}
			}
		}
	}

	if eocdOffset == -1 {
		return 0, 0, fmt.Errorf("not a zip file")
	}

	// Parse EOCD
	var eocd zipEOCD
	buf := bytes.NewReader(data[4:]) // skip magic (4 bytes)
	binary.Read(buf, binary.LittleEndian, &eocd.DiskNumber)
	binary.Read(buf, binary.LittleEndian, &eocd.CDDiskNumber)
	binary.Read(buf, binary.LittleEndian, &eocd.CDRecordsOnDisk)
	binary.Read(buf, binary.LittleEndian, &eocd.CDRecords)
	binary.Read(buf, binary.LittleEndian, &eocd.CDSize)
	binary.Read(buf, binary.LittleEndian, &eocd.CDOffset)
	binary.Read(buf, binary.LittleEndian, &eocd.CommentLength)

	cdNum := uint64(eocd.CDRecords)
	cdSize := uint64(eocd.CDSize)
	cdOffset := uint64(eocd.CDOffset)

	// Check for ZIP64
	if eocd.CDRecords == 0xffff || eocd.CDSize == 0xffffffff || eocd.CDOffset == 0xffffffff {
		// Read ZIP64 EOCD locator
		eocd64LocatorOffset := eocdOffset - zip64LocatorSize
		if eocd64LocatorOffset < 0 {
			return 0, 0, fmt.Errorf("unexpected eocd64_locator_offset")
		}

		locatorData, err := reader.Read(eocd64LocatorOffset, zip64LocatorSize)
		if err != nil {
			return 0, 0, err
		}

		if binary.LittleEndian.Uint32(locatorData[0:4]) != zip64LocatorMagic {
			return 0, 0, fmt.Errorf("unexpected EOCD64Locator magic")
		}

		var locator zip64Locator
		buf = bytes.NewReader(locatorData[4:]) // skip magic (4 bytes)
		binary.Read(buf, binary.LittleEndian, &locator.DiskNumber)
		binary.Read(buf, binary.LittleEndian, &locator.EOCDOffset)
		binary.Read(buf, binary.LittleEndian, &locator.TotalDisks)

		// Read ZIP64 EOCD
		eocd64Data, err := reader.Read(int64(locator.EOCDOffset), zip64EOCDSize)
		if err != nil {
			return 0, 0, err
		}

		if binary.LittleEndian.Uint32(eocd64Data[0:4]) != zip64EOCDMagic {
			return 0, 0, fmt.Errorf("unexpected EOCD64 magic")
		}

		var eocd64 zip64EOCD
		buf = bytes.NewReader(eocd64Data[4:]) // skip magic (4 bytes)
		binary.Read(buf, binary.LittleEndian, &eocd64.RecordSize)
		binary.Read(buf, binary.LittleEndian, &eocd64.VersionMadeBy)
		binary.Read(buf, binary.LittleEndian, &eocd64.VersionNeeded)
		binary.Read(buf, binary.LittleEndian, &eocd64.DiskNumber)
		binary.Read(buf, binary.LittleEndian, &eocd64.CDDiskNumber)
		binary.Read(buf, binary.LittleEndian, &eocd64.CDRecordsOnDisk)
		binary.Read(buf, binary.LittleEndian, &eocd64.CDRecords)
		binary.Read(buf, binary.LittleEndian, &eocd64.CDSize)
		binary.Read(buf, binary.LittleEndian, &eocd64.CDOffset)

		cdNum = eocd64.CDRecords
		cdSize = eocd64.CDSize
		cdOffset = eocd64.CDOffset
	}

	// Read central directory
	cdData, err := reader.Read(int64(cdOffset), int(cdSize))
	if err != nil {
		return 0, 0, err
	}

	nameBytes := []byte(name)
	var lfhOffset int64 = -1
	var fileEntrySize int64

	// Parse central directory entries
	pos := 0
	for i := uint64(0); i < cdNum && pos < len(cdData); i++ {
		if pos+zipCDFHSize > len(cdData) {
			break
		}

		if binary.LittleEndian.Uint32(cdData[pos:pos+4]) != zipCDFHMagic {
			return 0, 0, fmt.Errorf("invalid CD magic")
		}

		var cdfh zipCDFH
		buf = bytes.NewReader(cdData[pos+4:]) // skip magic (4 bytes)
		binary.Read(buf, binary.LittleEndian, &cdfh.VersionMadeBy)
		binary.Read(buf, binary.LittleEndian, &cdfh.VersionNeeded)
		binary.Read(buf, binary.LittleEndian, &cdfh.Flags)
		binary.Read(buf, binary.LittleEndian, &cdfh.CompressionMethod)
		binary.Read(buf, binary.LittleEndian, &cdfh.ModTime)
		binary.Read(buf, binary.LittleEndian, &cdfh.ModDate)
		binary.Read(buf, binary.LittleEndian, &cdfh.CRC32)
		binary.Read(buf, binary.LittleEndian, &cdfh.CompressedSize)
		binary.Read(buf, binary.LittleEndian, &cdfh.UncompressedSize)
		binary.Read(buf, binary.LittleEndian, &cdfh.FileNameLength)
		binary.Read(buf, binary.LittleEndian, &cdfh.ExtraFieldLength)
		binary.Read(buf, binary.LittleEndian, &cdfh.FileCommentLength)
		binary.Read(buf, binary.LittleEndian, &cdfh.DiskNumberStart)
		binary.Read(buf, binary.LittleEndian, &cdfh.InternalFileAttrs)
		binary.Read(buf, binary.LittleEndian, &cdfh.ExternalFileAttrs)
		binary.Read(buf, binary.LittleEndian, &cdfh.LocalHeaderOffset)

		entrySize := zipCDFHSize + int(cdfh.FileNameLength) + int(cdfh.ExtraFieldLength) + int(cdfh.FileCommentLength)
		if pos+entrySize > len(cdData) {
			break
		}

		fileName := cdData[pos+zipCDFHSize : pos+zipCDFHSize+int(cdfh.FileNameLength)]

		if bytes.Equal(fileName, nameBytes) {
			if cdfh.CompressionMethod != zipStored {
				return 0, 0, fmt.Errorf("target not stored: compression method %d", cdfh.CompressionMethod)
			}

			lfhOffset = int64(cdfh.LocalHeaderOffset)
			fileEntrySize = int64(cdfh.UncompressedSize)

			// Handle ZIP64 extensions
			if cdfh.UncompressedSize == 0xffffffff || cdfh.CompressedSize == 0xffffffff || cdfh.LocalHeaderOffset == 0xffffffff {
				extraPos := pos + zipCDFHSize + int(cdfh.FileNameLength)
				extraEnd := extraPos + int(cdfh.ExtraFieldLength)

				for extraPos < extraEnd-4 {
					headerID := binary.LittleEndian.Uint16(cdData[extraPos : extraPos+2])
					fieldSize := binary.LittleEndian.Uint16(cdData[extraPos+2 : extraPos+4])

					if extraPos+4+int(fieldSize) > extraEnd {
						break
					}

					if headerID == 1 { // ZIP64 extension
						extData := cdData[extraPos+4 : extraPos+4+int(fieldSize)]
						extPos := 0

						if cdfh.UncompressedSize == 0xffffffff && extPos+8 <= len(extData) {
							fileEntrySize = int64(binary.LittleEndian.Uint64(extData[extPos : extPos+8]))
							extPos += 8
						}
						if cdfh.CompressedSize == 0xffffffff && extPos+8 <= len(extData) {
							extPos += 8 // skip compressed size
						}
						if cdfh.LocalHeaderOffset == 0xffffffff && extPos+8 <= len(extData) {
							lfhOffset = int64(binary.LittleEndian.Uint64(extData[extPos : extPos+8]))
							extPos += 8
						}
					}

					extraPos += 4 + int(fieldSize)
				}
			}
			break
		}

		pos += entrySize
	}

	if lfhOffset == -1 {
		return 0, 0, fmt.Errorf("target not found: %s", name)
	}

	// Read local file header
	fhData, err := reader.Read(lfhOffset, zipFHSize)
	if err != nil {
		return 0, 0, err
	}

	if binary.LittleEndian.Uint32(fhData[0:4]) != zipFHMagic {
		return 0, 0, fmt.Errorf("unexpected file header magic")
	}

	var fh zipFH
	buf = bytes.NewReader(fhData[4:]) // skip magic (4 bytes)
	binary.Read(buf, binary.LittleEndian, &fh.VersionNeeded)
	binary.Read(buf, binary.LittleEndian, &fh.Flags)
	binary.Read(buf, binary.LittleEndian, &fh.CompressionMethod)
	binary.Read(buf, binary.LittleEndian, &fh.ModTime)
	binary.Read(buf, binary.LittleEndian, &fh.ModDate)
	binary.Read(buf, binary.LittleEndian, &fh.CRC32)
	binary.Read(buf, binary.LittleEndian, &fh.CompressedSize)
	binary.Read(buf, binary.LittleEndian, &fh.UncompressedSize)
	binary.Read(buf, binary.LittleEndian, &fh.FileNameLength)
	binary.Read(buf, binary.LittleEndian, &fh.ExtraFieldLength)

	realOffset := lfhOffset + zipFHSize + int64(fh.FileNameLength) + int64(fh.ExtraFieldLength)

	return realOffset, fileEntrySize, nil
}
