package dumper

import (
	"encoding/binary"
	"fmt"

	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/common/metadata"
	"github.com/xishang0128/payload-dumper-go/common/ziputil"

	"google.golang.org/protobuf/proto"
)

// ExtractMetadata extracts and returns the metadata from the payload.
func (d *Dumper) ExtractMetadata() ([]byte, error) {
	metadataPath := "META-INF/com/android/metadata"
	offset, size, err := ziputil.GetStoredEntryOffset(d.payloadFile, metadataPath)
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToExtractMetadata, metadataPath, err)
	}

	data, err := d.payloadFile.Read(offset, int(size))
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadMetadata, err)
	}

	return data, nil
}

func (d *Dumper) parseMetadata() error {
	headLen := 4 + 8 + 8 + 4
	buffer, err := d.payloadFile.Read(d.baseOffset, headLen)
	if err != nil {
		return err
	}

	if len(buffer) != headLen {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorInsufficientDataForHeader)
	}

	magic := string(buffer[:4])
	if magic != "CrAU" {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorInvalidMagic, magic)
	}

	fileFormatVersion := binary.BigEndian.Uint64(buffer[4:12])
	if fileFormatVersion != 2 {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnsupportedFileFormat, fileFormatVersion)
	}

	manifestSize := binary.BigEndian.Uint64(buffer[12:20])
	metadataSignatureSize := uint32(0)

	if fileFormatVersion > 1 {
		metadataSignatureSize = binary.BigEndian.Uint32(buffer[20:24])
	}

	fp := d.baseOffset + int64(headLen)

	manifestData, err := d.payloadFile.Read(fp, int(manifestSize))
	if err != nil {
		return err
	}
	fp += int64(manifestSize)

	fp += int64(metadataSignatureSize)

	d.dataOffset = fp
	d.manifest = &metadata.DeltaArchiveManifest{}

	if err := proto.Unmarshal(manifestData, d.manifest); err != nil {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToParseManifest, err)
	}

	d.blockSize = d.manifest.GetBlockSize()
	if d.blockSize == 0 {
		d.blockSize = 4096
	}

	return nil
}
